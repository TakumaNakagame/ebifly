FROM golang:1.26

WORKDIR /app

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8080

CMD ["go", "run", "./cmd/server", "-db", "/data/planning-poker.db", "-addr", ":8080"]
