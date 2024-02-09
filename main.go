package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"google.golang.org/api/sheets/v4"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

type req struct {
	Id int `form:"data[FIELDS][ID]"`
}

type deal struct {
	Result struct {
		Comment string `json:"COMMENTS"`
	} `json:"result"`
}

type contact struct {
	Result []struct {
		Id int `json:"CONTACT_ID"`
	} `json:"result"`
}

type contactData struct {
	Result struct {
		Name       string `json:"NAME"`
		SecondName string `json:"SECOND_NAME"`
		LastName   string `json:"LAST_NAME"`
		Phone      []struct {
			Value string `json:"VALUE"`
		} `json:"PHONE"`
	} `json:"result"`
}

func requestJson(client *http.Client, url string, id int, str interface{}) error {
	r, err := client.Get(fmt.Sprintf(url, id))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	_ = json.NewDecoder(r.Body).Decode(str)
	//fmt.Println(str)
	return nil
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	r := gin.Default()
	ctx := context.Background()

	b, err := os.ReadFile("cred.json")
	if err != nil {
		log.Fatalln(err)
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalln(err)
	}
	sheet_client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(sheet_client))
	if err != nil {
		log.Fatalln(err)
	}

	db, err := sql.Open("sqlite3", "store.db")
	if err != nil {
		log.Fatalln(err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS bills (id INTEGER PRIMARY KEY, fio TEXT, phone TEXT, comment TEXT);")
	if err != nil {
		log.Fatalln(err)
	}
	var client = &http.Client{Timeout: 10 * time.Second}
	r.POST("/api/in", func(c *gin.Context) {
		var t req
		var d deal
		var co contact
		var da contactData
		err := c.ShouldBind(&t)
		if err != nil {
			log.Println(err)
		}

		err = requestJson(client, "https://b24-s6iq27.bitrix24.ru/rest/1/kx3zweb2frb55ovf/crm.deal.get.json?id=%d", t.Id, &d)
		if err != nil {
			log.Println(err)
		}
		err = requestJson(client, "https://b24-s6iq27.bitrix24.ru/rest/1/ttf10q2b684yfawj/crm.deal.contact.items.get.json?id=%d", t.Id, &co)
		if err != nil {
			log.Println(err)
		}
		err = requestJson(client, "https://b24-s6iq27.bitrix24.ru/rest/1/60ys54rua0omrsgx/crm.contact.get.json?id=%d", co.Result[0].Id, &da)
		if err != nil {
			log.Println(err)
		}
		fio := fmt.Sprintf("%s %s %s", da.Result.SecondName, da.Result.Name, da.Result.LastName)
		fmt.Println("ФИО:", fio)
		fmt.Println("Телефон:", da.Result.Phone[0].Value)
		fmt.Println("Комментарий:", d.Result.Comment)

		records := [][]interface{}{{fio, da.Result.Phone[0].Value, d.Result.Comment}}
		values := &sheets.ValueRange{Values: records}

		srv.Spreadsheets.Values.Append("1zpmABo7EVj-Bfy0L9ZGw0zA-jfpDOgJrQU7nyWGM1Z8", "A:C", values).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
		_, err = db.Exec("INSERT INTO bills(fio,phone,comment) VALUES ($1, $2, $3)", fio, da.Result.Phone[0].Value, d.Result.Comment)
		if err != nil {
			log.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"ok": "ya",
		})
	})
	r.Run("0.0.0.0:2727")
}
