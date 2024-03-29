FROM golang:latest
WORKDIR /bitrix
COPY go.mod go.sum /bitrix/
RUN go mod download
COPY main.go cred.json token.json /bitrix/
EXPOSE 2727
RUN apt update
RUN apt install sqlite3
RUN go build .

CMD ["./awesomeProject"]