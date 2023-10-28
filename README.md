## Install

A sample of the `docker-compose.yml` file:

```
  postgres:
    image: "postgres:9.6"
    container_name: "postgres"
    ports:
      - 5432:5432
    volumes:
      - /srv/docker/pgsql/:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=password
    restart: always

  scrotter:
    build: ./omiewatcher
    image: omiewatcher:compose
    environment:
      - MATRIX_SERVER=
      - MATRIX_USER=
      - MATRIX_TOKEN=
      - MATRIX_ROOM=
      - PG_HOST=
      - PG_DB=
      - PG_USER=
      - PG_PASSWORD=
    links:
      - postgres
    restart: always
```

Setup the database for dev

```
CREATE DATABASE omiewatcher;
CREATE USER omiewatcher WITH PASSWORD 'omiewatcher';
GRANT ALL PRIVILEGES ON DATABASE "omiewatcher" to omiewatcher;
ALTER DATABASE omiewatcher OWNER TO omiewatcher;
```

