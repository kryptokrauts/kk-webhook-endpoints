package main

import (
	"fmt"
	"net/http"
	"os"

	"gopkg.in/kryptokrauts/webhooks.v6/pepo"
)

const (
	pepoPath = "/pepo"
)

func main() {
	pepoHook, _ := pepo.New(pepo.Options.Secret(os.Getenv("PEPO_SECRET")))

	http.HandleFunc(pepoPath, func(w http.ResponseWriter, r *http.Request) {
		payload, err := pepoHook.Parse(r)
		if err != nil {
			// handle error
			if err == pepo.ErrMissingPepoSignatureHeader {
				http.Error(w, err.Error(), 403)
			} else if err == pepo.ErrParsingPayload {
				http.Error(w, err.Error(), 400)
			} else if err == pepo.ErrInvalidHTTPMethod {
				http.Error(w, err.Error(), 405)
			} else {
				http.Error(w, err.Error(), 500)
			}
		} else {
			pepoEvent := payload.(pepo.EventPayload)
			switch pepoEvent.Topic {
			case "video/update":
				fmt.Println("received video update")
				// TODO check if video exists in our database
				exists := true
				if exists {
					if pepoEvent.Data.Activity.Video.Status == "DELETED" {
						// TODO delete entry from database
						fmt.Printf("Video with id '%d' deleted\n", pepoEvent.Data.Activity.Video.ID)
					} else {
						// TODO update entry in database
						fmt.Printf("Video with id '%d' updated\n", pepoEvent.Data.Activity.Video.ID)
					}
				} else {
					// TODO create entry in database
					fmt.Printf("Video with id '%d' created\n", pepoEvent.Data.Activity.Video.ID)
				}
			case "video/contribution":
				fmt.Println("received contribution =)")
			}
		}
	})

	http.ListenAndServe(":3000", nil)
}
