// Automating user accounts us technically against TOS - USE AT YOUR OWN RISK IF YOU WANT
// Automating user accounts us technically against TOS - USE AT YOUR OWN RISK IF YOU WANT
// Automating user accounts us technically against TOS - USE AT YOUR OWN RISK IF YOU WANT

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cavaliergopher/grab/v3"
)

// set http proxy for grab.Get
// comment the init method if you don't want to use a proxy
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
	tasks = []*Task{
		// demo tasks, change to the real params
		NewTask(`abc`, `123`),
		NewTask(`xyz`, `789`),
	}
)

func main() {
	if len(tasks) == 0 {
		log.Fatalln("no tasks")
	}
	session, err := discordgo.New(auth)
	if err != nil {
		log.Fatalln(fmt.Sprintf("error creating discord session: %v", err))
	}
	defer func(session *discordgo.Session) { _ = session.Close() }(session)
	for _, task := range tasks {
		task.Execute(session)
	}
	log.Println("done")
}

type Options func(t *Task)

func DebugMode(val bool) Options      { return func(t *Task) { t.debug = val } }
func WithBeforeID(val string) Options { return func(t *Task) { t.beforeID = val } }
func WithMaxLoop(val uint) Options    { return func(t *Task) { t.maxLoop = val } }
func WithSize(val uint) Options       { return func(t *Task) { t.size = val } }
func WithRetry(val uint) Options      { return func(t *Task) { t.retry = val } }
func WithAttachmentFilter(filter func(*discordgo.MessageAttachment) bool) Options {
	return func(t *Task) { t.attachmentFilter = filter }
}

type Task struct {
	GuildName string // discord guild name, like "Discord Developers", is used to build the output dirname
	ChannelID string // the snowflake ID of the channel, used to climb the messages

	debug            bool   // verbose logs
	beforeID         string // message ID, if provided all messages returned will be before given ID
	maxLoop          uint   // means the maximum pages you want to scan
	size             uint
	retry            uint
	attachmentFilter func(*discordgo.MessageAttachment) bool

	saveDir     string
	currentLoop uint
}

func NewTask(guildName, channelID string, opts ...Options) *Task {
	t := &Task{
		GuildName: guildName,
		ChannelID: channelID,

		beforeID:         "",
		maxLoop:          5,
		saveDir:          filepath.Join(outputAbsDir, fmt.Sprintf("DC-%s-%s", guildName, channelID)),
		currentLoop:      0,
		size:             100,
		retry:            5,
		attachmentFilter: defaultAttachmentFilter,

		debug: false,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Task) Execute(session *discordgo.Session) {
	if t.GuildName == "" || t.ChannelID == "" {
		logError(t, "invalid argument: GuildName and ChannelID must be set")
		return
	}

	_ = os.MkdirAll(t.saveDir, os.ModePerm)

	for t.currentLoop < t.maxLoop {
		messages, err := session.ChannelMessages(t.ChannelID, int(t.size), t.beforeID, "", "")
		if err != nil {
			logError(t, fmt.Sprintf("!! error fetching channel messages: %s", err))
			break
		}
		if len(messages) == 0 {
			logInfo(t, "no more message, task finished")
			return
		}

		for _, message := range messages {
			t.beforeID = message.ID
			t.processAttachments(message.Attachments)
			t.processEmbeds(message)
			t.processMessage(message.ReferencedMessage)
			t.processMessageSnapshot(message.MessageSnapshots)
		}

		t.currentLoop++
		logInfo(t, fmt.Sprintf("next scan start at message id %s, wait 2 seconds to start...", t.beforeID))
		time.Sleep(2 * time.Second)
	}

	logInfo(t, fmt.Sprintf("reach the max loop, stop at message id %s", t.beforeID))
}

func (t *Task) download(resource, saveTo, mURL string, retry uint) (err error) {
	if retry == 0 {
		logInfo(t, "downloading "+mURL)
	} else {
		logInfo(t, fmt.Sprintf("(retry %d)", retry)+" downloading "+mURL)
	}

	tmpPath := saveTo + ".tmp"
	_ = os.Remove(tmpPath)
	if _, err = grab.Get(tmpPath, mURL); err == nil {
		_ = os.Rename(tmpPath, saveTo)
		logInfo(t, fmt.Sprintf("downloaded %s: %s", resource, saveTo))
		return nil
	}
	return err
}

func (t *Task) processAttachments(attachments []*discordgo.MessageAttachment) {
	for _, attachment := range attachments {
		if ok := t.attachmentFilter(attachment); !ok {
			continue
		}

		saveTo := filepath.Join(t.saveDir, fmt.Sprintf("%s_%s", attachment.ID, attachment.Filename))
		if ok, _ := isPathExist(saveTo); ok {
			if t.debug {
				logWarn(t, fmt.Sprintf("skip exist attachment: %s", saveTo))
			}
			continue
		}

		var err error
		for retry := uint(0); retry < t.retry; retry++ {
			if err = t.download("attachment", saveTo, attachment.URL, retry); err == nil {
				break
			}
		}
		if err == nil {
			continue
		}
		for retry := uint(0); retry < t.retry; retry++ {
			if err = t.download("attachment", saveTo, attachment.ProxyURL, retry); err == nil {
				break
			}
		}
		if err != nil {
			logWarn(t, fmt.Sprintf("fail to download attachment: %s", saveTo))
			logWarn(t, fmt.Sprintf("    - url: %s", attachment.URL))
			logWarn(t, fmt.Sprintf("    - proxy_url: %s", attachment.ProxyURL))
		}
	}
}

func (t *Task) processEmbeds(message *discordgo.Message) {
	for i, embed := range message.Embeds {
		if embed == nil {
			continue
		}
		if embed.Image != nil {
			mURL := embed.Image.URL
			if strings.Contains(mURL, "cdn.discordapp.com") && strings.Contains(mURL, "?") {
				mURL = mURL[:strings.Index(mURL, "?")]
			}
			t.processEmbedMedia(i, message.ID, mURL, embed.Image.ProxyURL, "eb_res.jpg")
		}
		if embed.Thumbnail != nil {
			t.processEmbedMedia(i, message.ID, embed.Thumbnail.URL, embed.Thumbnail.ProxyURL, "eb_res.jpg")
		}
		if embed.Video != nil {
			t.processEmbedMedia(i, message.ID, embed.Video.URL, "", "eb_res.mp4")
		}
	}
}

func (t *Task) processEmbedMedia(index int, messageID, mURL, proxyURL, defaultFilename string) {
	saveTo := t.embedMediaSaveTo(index, messageID, mURL, proxyURL, defaultFilename)
	if ok, _ := isPathExist(saveTo); ok {
		if t.debug {
			logWarn(t, fmt.Sprintf("skip exist embed media: %s", saveTo))
		}
		return
	}

	var err error
	for retry := uint(0); retry < t.retry; retry++ {
		if err = t.download("embed media", saveTo, mURL, retry); err == nil {
			break
		}
	}
	if err == nil {
		return
	}
	for retry := uint(0); retry < t.retry; retry++ {
		if err = t.download("embed media", saveTo, proxyURL, retry); err == nil {
			break
		}
	}
	if err != nil {
		logWarn(t, fmt.Sprintf("fail to download embed media: %s", saveTo))
		logWarn(t, fmt.Sprintf("    - url: %s", mURL))
		logWarn(t, fmt.Sprintf("    - proxy_url: %s", proxyURL))
	}
}

func (t *Task) processMessage(message *discordgo.Message) {
	if message != nil && len(message.Attachments) > 0 {
		t.processAttachments(message.Attachments)
	}
}

func (t *Task) processMessageSnapshot(snapshots []discordgo.MessageSnapshot) {
	for _, snapshot := range snapshots {
		t.processMessage(snapshot.Message)
	}
}

func (t *Task) embedMediaSaveTo(index int, messageID, mURL, proxyURL, defaultFilename string) (new string) {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s_%s", messageID, mURL)))
	m := hex.EncodeToString(h.Sum(nil))

	ext := filepath.Ext(mURL)
	if ext == "" {
		ext = filepath.Ext(proxyURL)
	}
	if ext == "" {
		ext = filepath.Ext(defaultFilename)
	}

	re := regexp.MustCompile(`\.[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*`)
	ext = re.FindString(ext)

	return filepath.Join(t.saveDir, fmt.Sprintf("%s_embed_%d_%s%s", messageID, index, m, ext))
}

func defaultAttachmentFilter(attachment *discordgo.MessageAttachment) bool {
	if attachment == nil {
		return false
	}
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

func logInfo(task *Task, message string) {
	taskName := fmt.Sprintf("[%s.%s]", task.GuildName, task.ChannelID)
	log.Printf(taskName + " " + message)
}

func logWarn(task *Task, message string) {
	taskName := fmt.Sprintf("[%s.%s]", task.GuildName, task.ChannelID)
	log.Printf(taskName + " " + message)
}

func logError(task *Task, message string) {
	taskName := fmt.Sprintf("[%s.%s]", task.GuildName, task.ChannelID)
	log.Printf(taskName + " " + message)
}
