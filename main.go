package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"gopkg.in/kryptokrauts/webhooks.v6/pepo"
)

const (
	pepoPath = "/pepo"
)

func main() {
	pepoHook, _ := pepo.New(pepo.Options.Secret(os.Getenv("PEPO_SECRET")))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+os.Getenv("MONGO_HOST")+":"+os.Getenv("MONGO_PORT")))
	if err != nil {
		panic(err)
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic(err)
	}

	discordSession, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		panic(err)
	}

	videoUpdates := client.Database("pepo").Collection("video-updates")
	videoContributions := client.Database("pepo").Collection("video-contributions")

	http.HandleFunc(pepoPath, func(w http.ResponseWriter, r *http.Request) {
		payload, err := pepoHook.Parse(r)
		if err != nil {
			// handle error
			if err == pepo.ErrMissingSignatureHeader || err == pepo.ErrMissingTimestampHeader || err == pepo.ErrMissingVersionHeader || err == pepo.ErrHMACVerificationFailed {
				http.Error(w, err.Error(), 403)
			} else if err == pepo.ErrParsingPayload {
				http.Error(w, err.Error(), 400)
			} else if err == pepo.ErrInvalidHTTPMethod {
				http.Error(w, err.Error(), 405)
			} else {
				http.Error(w, err.Error(), 500)
			}
		} else {
			var databaseErr error
			var discordMsg string
			pepoEvent := payload.(pepo.EventPayload)
			switch pepoEvent.Topic {
			case "video/update":
				fmt.Println("received update event")
				_, databaseErr = videoUpdates.InsertOne(context.Background(), pepoEvent)
				if pepoEvent.Data.Activity.Video.Status == "ACTIVE" {
					discordMsg = fmt.Sprintf("**%s** created/updated a video: %s",
						pepoEvent.Data.Users[strconv.FormatInt(pepoEvent.Data.Activity.ActorID, 10)].Name,
						pepoEvent.Data.Activity.Video.URL)
				} else {
					discordMsg = fmt.Sprintf("**%s** deleted video with id: %d",
						pepoEvent.Data.Users[strconv.FormatInt(pepoEvent.Data.Activity.ActorID, 10)].Name,
						pepoEvent.Data.Activity.Video.ID)
				}
			case "video/contribution":
				fmt.Println("received contribution event")
				var lastContributionEvent pepo.EventPayload
				opts := options.FindOne()
				opts.SetSort(bson.D{{Key: "created_at", Value: -1}})
				if err = videoContributions.FindOne(context.Background(), bson.M{"data.activity.video.id": pepoEvent.Data.Activity.Video.ID}, opts).Decode(&lastContributionEvent); err != nil {
					fmt.Println("previous contribution doesn't exist")
				}
				currentTotalContributionAmount, _ := strconv.ParseFloat(pepoEvent.Data.Activity.Video.TotalContributionAmount, 32)
				previousTotalContributionAmount, _ := strconv.ParseFloat(lastContributionEvent.Data.Activity.Video.TotalContributionAmount, 32)
				_, databaseErr = videoContributions.InsertOne(context.Background(), pepoEvent)
				discordMsg = fmt.Sprintf("**%s** contributed **%d Pepo** to the video of **%s**: %s",
					pepoEvent.Data.Users[strconv.FormatInt(pepoEvent.Data.Activity.ActorID, 10)].Name,
					int(currentTotalContributionAmount-previousTotalContributionAmount),
					pepoEvent.Data.Users[strconv.FormatInt(pepoEvent.Data.Activity.Video.CreatorID, 10)].Name,
					pepoEvent.Data.Activity.Video.URL)
			}
			if databaseErr != nil {
				fmt.Println(err)
				fmt.Println("error writing event data into database: " + toJSON(payload))
			}
			sendDiscordMessage(discordSession, discordMsg)
		}
	})

	http.ListenAndServe(":3000", nil)
}

func toJSON(i interface{}) string {
	s, _ := json.Marshal(i)
	return string(s)
}

func sendDiscordMessage(session *discordgo.Session, message string) {
	session.Open()
	session.ChannelMessageSend(os.Getenv("DISCORD_CHANNEL_ID"), message)
	session.Close()
}
