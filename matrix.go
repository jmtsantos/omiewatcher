package main

import (
	"bytes"

	"github.com/matrix-org/gomatrix"

	log "github.com/sirupsen/logrus"
)

func UploadImageToMatrix(image []byte) (response *gomatrix.RespMediaUpload, err error) {
	reader := bytes.NewReader(image)

	if response, err = MatrixClient.UploadToContentRepo(reader, "image/png", reader.Size()); err != nil {
		log.WithError(err).Errorln("error sending matrix message %s", err)
		return response, err
	}

	return response, err
}

// SendImage sends a matrix notification
func SendImage(message, contentURI string) (err error) {
	if _, err = MatrixClient.SendImage(MatrixRoom, message, contentURI); err != nil {
		log.WithError(err).Errorln("error sending matrix message %s", err)
		return
	}

	return
}

// SendMatrixNotification sends a matrix notification
func SendMatrixNotification(message string) (err error) {
	msg := RoomMsg{
		MsgType:       "m.text",
		Format:        "org.matrix.custom.html",
		Body:          message,
		FormattedBody: message,
	}

	if _, err = MatrixClient.SendMessageEvent(MatrixRoom, "m.room.message", msg); err != nil {
		log.WithError(err).Errorln("error sending matrix message %s", err)
		return
	}

	return
}

// RoomMsg message structure
type RoomMsg struct {
	MsgType       string `json:"msgtype"`
	Format        string `json:"format"`
	Body          string `json:"body"`
	FormattedBody string `json:"formatted_body"`
}
