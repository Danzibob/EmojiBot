package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-server/model"
)

const (
	// HOST is the domain (and port) for the Mattermost Server
	HOST = "york.codesigned.co.uk"

	BOT_USERNAME = "york-55-bot"
	BOT_PASSWORD = "cible1"

	TEAM_NAME = "uni-of-york"

	// CHANNEL_NAME should be your username
	CHANNEL_NAME = "york-55"
)

var client *model.Client4
var webSocketClient *model.WebSocketClient
var channel *model.Channel
var bot *model.User

func main() {
	client = model.NewAPIv4Client("https://" + HOST)

	// Load the emoji file
	emoji, _ := readLines("emojiList")

	// Login as the bot user
	var resp *model.Response
	bot, resp = client.Login(BOT_USERNAME, BOT_PASSWORD)

	// Check if there was a login error
	if resp.Error != nil {
		fmt.Println("Login error:", resp.Error)
		os.Exit(1)
	}

	// Team
	team, resp := client.GetTeamByName(TEAM_NAME, "")
	if resp.Error != nil {
		fmt.Println("Error finding team:", resp.Error)
		os.Exit(1)
	}

	// Find the channel ID
	channel, resp = client.GetChannelByName(CHANNEL_NAME, team.Id, "")
	if resp.Error != nil {
		fmt.Println("Error finding channel:", resp.Error)
		os.Exit(1)
	}

	// Connect to Mattermost websocket
	var err *model.AppError
	webSocketClient, err = model.NewWebSocketClient("wss://"+HOST, client.AuthToken)

	// If there's an error, just exit
	if err != nil {
		fmt.Println("Web Socket Error:", err)
		os.Exit(1)
	}

	// Start the client listening
	webSocketClient.Listen()
	fmt.Println("Listening for messages on " + CHANNEL_NAME)

	// Forever loop waiting for messages on the EventChannel
	for {
		select {
		case resp := <-webSocketClient.EventChannel:
			HandleWebSocketResponse(resp, &emoji)
		}
	}
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
// Credit: https://stackoverflow.com/questions/5884154/read-text-file-into-string-array-and-write
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// HandleWebSocketResponse receives all events from the web socket
func HandleWebSocketResponse(event *model.WebSocketEvent, emoji *[]string) {
	// Filter out all other channels
	if event.Broadcast.ChannelId != channel.Id {
		return
	}

	// Only respond to posted messages
	// More event types here:
	// https://github.com/mattermost/mattermost-server/blob/master/model/websocket_message.go#L12
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))

	// If no issues, then continue
	if post != nil {
		// Ensure this isn't a bot message
		if post.UserId == bot.Id {
			return
		}

		fmt.Println("Received message, responding...")

		// Get the text message from the post
		msg := post.Message
		msg = " " + strings.ToLower(msg) + " "
		// For each emoji
		for _, e := range *emoji {
			// Compile a regex to find that emoji
			r, err := regexp.Compile("\\b(" + e + ")\\b")
			// If this was successful then
			if err == nil {
				// Make the string a markdown emoji
				msg = r.ReplaceAllString(msg, ":$1:")
			} else {
				// Otherwise, print the error and continue
				fmt.Println(err)
			}
		}

		// Moar :b:
		words := strings.Fields(msg)
		r, _ := regexp.Compile("b")
		for i, w := range words {
			if w[0] != ':' {
				// Unfortunately, these were only parsed as emoji if I added the spaces
				words[i] = r.ReplaceAllString(w, " :b: ")
			}
		}

		msg = strings.Join(words, " ")

		// Send the message to the channel as a reply to this post
		print(msg)
		SendMessage(msg, post.Id)
	}
}

// SendMessage creates a new post on the channel as a reply
func SendMessage(msg string, replyToId string) {
	// Create a post
	post := &model.Post{}
	post.ChannelId = channel.Id
	post.Message = msg

	// Setting root id makes this a reply
	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		fmt.Println("Post error:", resp.Error)
	}
}
