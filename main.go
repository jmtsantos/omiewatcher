package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/joho/godotenv"
	"github.com/matrix-org/gomatrix"
	"github.com/shopspring/decimal"

	"gorm.io/gorm"

	log "github.com/sirupsen/logrus"
)

var (
	Testing = false

	OmSearch = "https://www.omie.es/sites/default/files/dados/AGNO_2023/MES_%s/TXT/INT_MERCADO_DIARIO_MIN_MAX_1_%s_%s.TXT"

	config ConfigData

	EnvVars = []string{
		"MATRIX_SERVER", "MATRIX_USER", "MATRIX_TOKEN", "MATRIX_ROOM",
		"PG_HOST", "PG_DB", "PG_USER", "PG_PASSWORD",
	}

	MatrixClient *gomatrix.Client

	MatrixServer   string
	MatrixUsername string
	MatrixToken    string
	MatrixRoom     string
	PostgresHost   string
	PostgresDb     string
	PostgresUser   string
	PostgresPwd    string

	Db *gorm.DB

	AvgValue float64
)

func init() {
	var (
		err error
	)

	// Check and set all the env variables
	if Testing {
		if err = godotenv.Load(); err != nil {
			log.WithError(err).Fatalf("error loading .env file")
		}
	}

	for _, env := range EnvVars {
		if _, ok := os.LookupEnv(env); !ok {
			log.Fatalf("enviorment variable %s not set", env)
		}
	}

	MatrixServer = os.Getenv("MATRIX_SERVER")
	MatrixUsername = os.Getenv("MATRIX_USER")
	MatrixToken = os.Getenv("MATRIX_TOKEN")
	MatrixRoom = os.Getenv("MATRIX_ROOM")
	PostgresHost = os.Getenv("PG_HOST")
	PostgresDb = os.Getenv("PG_DB")
	PostgresUser = os.Getenv("PG_USER")
	PostgresPwd = os.Getenv("PG_PASSWORD")

	// matrix
	if MatrixClient, err = gomatrix.NewClient(MatrixServer, MatrixUsername, MatrixToken); err != nil {
		log.WithError(err).Fatalf("error creating matrix client")
	}

	// database
	if Db, err = NewDB(PostgresHost, PostgresUser, PostgresPwd, PostgresDb); err != nil {
		log.WithError(err).Fatalf("error opening the database")
	}

	// load config
	configRaw := Config{}
	if err = Db.Model(Config{}).First(&configRaw).Error; err != nil {
		config.MaxValue = 130.0
		configRaw.Data, _ = json.Marshal(config)
		Db.Create(&configRaw)
	}

	if err = json.Unmarshal(configRaw.Data, &config); err != nil {
		log.WithError(err).Fatalf("error loading config")
	}
}

func main() {
	log.Println("omiewatcher is running")

	// First run
	updateListings()

	s := gocron.NewScheduler(time.UTC)
	s.Every(1).Day().At("07:30").Do(updateListings)
	s.StartAsync()

	router := gin.Default()
	router.GET("/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"config": config})
	})
	router.POST("/config", func(c *gin.Context) {
		var (
			configTemp ConfigData
			err        error
		)
		if err = c.BindJSON(&configTemp); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		configRaw := Config{}
		Db.Model(Config{}).First(&configRaw)

		configRaw.Data, _ = json.Marshal(configTemp)
		config = configTemp
		Db.Save(&configRaw)

		c.JSON(http.StatusOK, gin.H{
			"message": "JSON saved successfully",
		})
	})
	router.GET("/last", func(c *gin.Context) {
		c.JSON(200, gin.H{"last_avg_value": AvgValue})
	})
	router.GET("/update", func(c *gin.Context) {
		updateListings()
		c.JSON(200, gin.H{"message": "updated"})
	})
	router.Run()
}

func updateListings() {
	var (
		data  []Entry
		value float64
		err   error
	)

	// Start the scrapper
	if data, err = getData(); err != nil || len(data) == 0 {
		log.WithFields(log.Fields{
			"error":   err,
			"entries": len(data),
		}).Errorln("error downloading data")
		SendMatrixNotification(fmt.Sprintf("<h3>omiewatcher</h3><bold>error downloading data</bold> %v", err))
		return
	}

	// Save everything to db
	for _, entry := range data {
		if err = Db.FirstOrCreate(&entry).Error; err != nil {
			log.WithError(err).Errorln("error adding listings to db")
			return
		}
	}

	// Do summary calculations
	if value, err = checkPriceTrend(); err != nil {
		log.WithError(err).Errorln("error adding listings to db")
		return
	}

	// Send matrix notification if it hits maximum
	if value > config.MaxValue {
		SendMatrixNotification(fmt.Sprintf(`<h3>omiewatcher</h3><p><bold>Price will hit hardcoded maximum: </bold>%0.2f</p>`, value))
	}

	log.Printf("scrape finished, average value %f", value)
}

func getData() (entries []Entry, err error) {
	var (
		res   *http.Response
		lines [][]string
	)

	// get the dates for the URL format
	now := time.Now()
	month := now.Format("01")
	firstDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("02_01_2006")
	nextMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	lastDay := nextMonth.Add(-time.Hour * 24).Format("02_01_2006")
	url := fmt.Sprintf(OmSearch, month, firstDay, lastDay)

	// Request the HTML page.
	if res, err = http.Get(url); err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("server status code not 200: %d", res.StatusCode)
	}

	reader := csv.NewReader(res.Body)
	reader.Comma = ';'
	reader.FieldsPerRecord = 5
	lineNum := 0
	for {
		lineNum++
		record, err := reader.Read()

		// skip the first two lines
		if lineNum < 3 {
			continue
		}

		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			if err != nil {
				log.Warnln(err)
				continue
			}
		}

		lines = append(lines, record)
	}

	for _, line := range lines {

		minimo, err := decimal.NewFromString(strings.ReplaceAll(line[1], ",", "."))
		if err != nil {
			continue
		}
		medio, err := decimal.NewFromString(strings.ReplaceAll(line[2], ",", "."))
		if err != nil {
			continue
		}
		maximo, err := decimal.NewFromString(strings.ReplaceAll(line[3], ",", "."))
		if err != nil {
			continue
		}
		fecha, err := time.Parse("02/01/06", line[0])
		if err != nil {
			continue
		}
		sumData := fmt.Sprintf("%s%s%s", line[1], line[2], line[3])

		entries = append(entries, Entry{
			ID:        fmt.Sprintf("%x", sha256.Sum256([]byte(sumData))),
			Fecha:     fecha,
			Min:       minimo,
			Avg:       medio,
			Max:       maximo,
			CreatedAt: time.Now(),
		})
	}

	return
}

func checkPriceTrend() (value float64, err error) {
	var prices []float64

	// Calculate the dates for the last 30 days
	currentTime := time.Now()
	endDate := currentTime
	startDate := currentTime.AddDate(0, 0, -29)

	if err = Db.Model(Entry{}).Select("avg").Where("fecha BETWEEN ? AND ?", startDate, endDate).Scan(&prices).Error; err != nil {
		return
	}

	movingAverage := ewma.NewMovingAverage(5) //=> returns a VariableEWMA with a decay of 2 / (5 + 1)

	// Calculate the 14 day trend
	for i := len(prices) - 1; i >= len(prices)-14; i-- {
		movingAverage.Add(prices[i])
	}

	value = movingAverage.Value()
	AvgValue = value

	return
}
