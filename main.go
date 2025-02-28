package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cavaliergopher/grab/v3"

	"github.com/AyakuraYuki/go-grab-discord-attachments/colors"
)

// set http proxy, if you don't want to use a proxy, comment the following lines
func init() {
	_ = os.Setenv("HTTP_PROXY", "http://127.0.0.1:7890")
	_ = os.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
}

// Config variables, change to what you need to grab and where you want to save.
var (
	// An absolute dir to save attachments, for example, the outputAbsDir is `/path/to/saves`,
	// groupName is `abc`, channelID is `123`, the final path is `/path/to/saves/DC-abc-123`.
	outputAbsDir = ``

	// auth is your Discord API authorization token
	//
	// to get the token for your personal account:
	//	1. open Discord in your web browser and login
	//	2. open any server or direct message channel
	//	3. press Ctrl+Alt+I or F12 to show developer tools
	//	4. navigate to the network tab
	//	5. press Ctrl+R to reload the page
	//	6. switch between random channels to trigger network requests
	//	7. search for a request that contains with `messages`
	//	8. select the `Headers` tab on the right
	//	9. scroll down to the `Request Headers` section
	//	10. copy the value of the `authorization` header
	// * Automating user accounts us technically against TOS - USE AT YOUR OWN RISK
	auth = ``

	// tasks for grabbing
	tasks = []taskDefinition{
		// demo tasks, change to the real params
		{groupName: `abc`, channelID: `123`, before: ``, maxLoop: 20},
		{groupName: `xyz`, channelID: `789`, before: ``, maxLoop: 20},
	}
)

func main() {
	if auth == "" {
		panic("auth required")
	}
	if len(tasks) == 0 {
		panic("tasks required")
	}

	session, err := discordgo.New(auth)
	if err != nil {
		panic(fmt.Errorf("error creating Discord session: %v", err))
	}
	defer func(bot *discordgo.Session) { _ = bot.Close() }(session)

	for i, task := range tasks {
		executeTask(session, i, task)
	}

	fmt.Println(colors.Green("done"))
}

func executeTask(session *discordgo.Session, i int, task taskDefinition) {
	no := i + 1
	if task.groupName == "" || task.channelID == "" {
		fmt.Println(colors.Yellow("** task no.%d with empty group name or channel id, skipped", no))
		return
	}

	before := task.before
	currentLoop := 0
	saveDir := filepath.Join(outputAbsDir, fmt.Sprintf("DC-%s-%s", task.groupName, task.channelID))
	_ = os.MkdirAll(saveDir, os.ModePerm)

	for currentLoop < task.maxLoop {
		messages, errChannelMessages := session.ChannelMessages(task.channelID, 100, before, "", "")
		if errChannelMessages != nil {
			fmt.Printf(colors.Red("!! error fetching channel messages: %v", errChannelMessages))
			break
		}
		if len(messages) == 0 {
			fmt.Println(colors.Green("no more messages, task no.%d finished", no))
			return
		}

		for _, message := range messages {
			before = message.ID
			if len(message.Attachments) == 0 {
				continue
			}
			for _, attachment := range message.Attachments {
				if ok := containsAcceptableAttachment(attachment); !ok {
					continue // skip unmatched attachment
				}

				absFilepath := filepath.Join(saveDir, fmt.Sprintf("%s_%s", attachment.ID, attachment.Filename))
				if ok, _ := isPathExist(absFilepath); ok {
					fmt.Println(colors.Blue("  - skip exist attachment: %s", absFilepath))
					continue
				}

				var errDownload error
				for i := 0; i < 5; i++ {
					fmt.Println(colors.Cyan("  - download attachment: %s", absFilepath))
					if _, errDownload = grab.Get(absFilepath, attachment.URL); errDownload == nil {
						break
					}
				}
				if errDownload != nil {
					for i := 0; i < 5; i++ {
						fmt.Println(colors.Cyan("  - download attachment using proxy_url: %s", absFilepath))
						if _, errDownload = grab.Get(absFilepath, attachment.ProxyURL); errDownload == nil {
							break
						}
					}
				}
				if errDownload != nil {
					fmt.Println(colors.Red("  - (skip) download attachment failed: %s", absFilepath))
				}
			}
		}

		currentLoop++
		fmt.Println(colors.Yellow("next scan start at message id %s, wait 5 seconds to start...", before))
		time.Sleep(5 * time.Second)
	}

	fmt.Println(colors.Green("[reach loop limit] task no.%d stop at message id %s", no, before))
}

func containsAcceptableAttachment(attachment *discordgo.MessageAttachment) bool {
	if attachment == nil {
		return false
	}

	// feel free to modify the following conditions

	ct := strings.ToLower(attachment.ContentType)
	if strings.HasPrefix(ct, "image") {
		return true
	}
	if strings.HasPrefix(ct, "video") {
		return true
	}

	return attachment.Width > 0 && attachment.Height > 0
}

func isPathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

type taskDefinition struct {
	groupName string // groupName is Discord group name, used to build the output dirname.
	channelID string // channelID is Discord channel snowflake id, used to climb the messages.

	// leave it empty to start from the latest message, or set it with the message snowflake id
	// if you want to start from a specified message
	before string

	// maxLoop means the maximum pages you want to scan, I recommend you to set this value
	// between 1 and 200.
	// Automating user accounts us technically against TOS - USE AT YOUR OWN RISK IF YOU WANT
	// TO SET IT OVER 200.
	maxLoop int
}
