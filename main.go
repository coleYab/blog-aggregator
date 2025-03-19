package main

import (
    "context"
    "database/sql"
    "encoding/xml"
    "fmt"
    "html"
    "http_clients/internal/config"
    "http_clients/internal/database"
    "io"
    "net/http"
    "os"

    _ "github.com/lib/pq"
)

type state struct {
    db *database.Queries
    cfg *config.Config
}

type RSSFeed struct {
    Channel struct {
        Title       string    `xml:"title"`
        Link        string    `xml:"link"`
        Description string    `xml:"description"`
        Item        []RSSItem `xml:"item"`
    } `xml:"channel"`
}

type RSSItem struct {
    Title       string `xml:"title"`
    Link        string `xml:"link"`
    Description string `xml:"description"`
    PubDate     string `xml:"pubDate"`
}

func (r *RSSItem)EscapeHtml() {
    r.Title = html.UnescapeString(r.Title)
    r.Description = html.UnescapeString(r.Description)
}

const feedUrl = "https://www.wagslane.dev/index.xml"
type command struct {
    name string
    args []string
}

type commandHandler func(*state, command)error

type commands struct {
    cmds map[string]commandHandler
}

// middleware
func protectCommands(handler func(s *state, user *database.User, c command)error) commandHandler {
    return func(s *state, c command) error {
        user, err := s.db.GetUserByName(context.Background(), s.cfg.CurrentUserName)
        if err != nil {
            return fmt.Errorf("User is not found")
        }

        return handler(s, &user, c)
    }
}

func fetchFeeds(ctx context.Context, feedUrl string) (*RSSFeed, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedUrl, nil)
    if err != nil {
        return nil, fmt.Errorf("ERROR: error occured while creating the request `%v`", err.Error())
    }

    req.Header.Add("User-Agent", "gator")

    res, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("ERROR: error while requesting the data `%v`", err.Error())
    }

    defer res.Body.Close()
    data, err := io.ReadAll(res.Body)
    if err != nil {
        return nil, fmt.Errorf("ERROR: error while reading the body `%v`", err.Error())
    }

    feed := &RSSFeed{}
    if err := xml.Unmarshal(data, feed); err != nil {
        return nil, fmt.Errorf("ERROR: error while Unmarshalling the result `%v`", err.Error())
    }

    for _, item := range feed.Channel.Item {
        item.EscapeHtml()
    }

    return feed, nil
}

func (c *commands)register(name string, ch commandHandler) {
    c.cmds[name] = ch
}

func (c *commands)run(s *state, cmd command) error {
    handler, ok := c.cmds[cmd.name]
    if !ok {
        return fmt.Errorf("The given command `%v` was not found", cmd.name)
    }

    return handler(s, cmd)
}

func feedsHandler(s *state, c command) error {
    // lets select all the feeds that we have
    feeds, err := s.db.SelectFeedsWithCreator(context.Background())
    if err != nil {
        return fmt.Errorf("Error: whlie selecting all the fields `%v`", err.Error())
    }

    i := 0
    for _, feed := range feeds {
        if i == 0 {
            fmt.Println("-- feeds records:")
        }
        fmt.Printf("--> Feed: [%v] \n\t- Name - [%v] \n\t- Url - [%v] \n\t- Created By - [%v]\n", i + 1, feed.Name, feed.Url, feed.UserName)
        i += 1
    }

    return nil
}
func resetHandler(s *state, c command) error {
    if err := s.db.DeleteAllUsers(context.Background()); err != nil {
        return fmt.Errorf("Error while trying to reset all the registered users")
    }

    fmt.Println("successfully reset the user database")

    return nil
}

func registerHandler(s *state, c command) error {
    if c.args == nil || len(c.args) == 0 {
        return fmt.Errorf("`register` command expects a single argument the `username`")
    }

    // check if the user exists perviously
    _, err := s.db.GetUserByName(context.Background(), c.args[0])
    if err == nil {
        // if the user exists
        return fmt.Errorf("User registeration failed because there was a user associated with this name `%v`", c.args[0])
    }

    user, err := s.db.CreateUser(context.Background(), c.args[0])
    if err != nil {
        return err
    }

    fmt.Printf("User with name `%v` has been created successfully at `%v`.\n", user.Name, user.CreatedAt.Time.String())

    return s.cfg.SetUser(user.Name)
}

func prettyPrint(rssFeed  *RSSFeed) {
    fmt.Println("-------- Rss Feed Channel Details -----------------")
    fmt.Println("@@ Feed Title: ", rssFeed.Channel.Title)
    fmt.Println("@@ Feed Link : ", rssFeed.Channel.Link)
    fmt.Println("@@ Feed Description : ", rssFeed.Channel.Description)
    if len(rssFeed.Channel.Item) == 0 {
        fmt.Println("No RSS FEED ITEMS")
        return
    }
    fmt.Println("-------- Rss Feed Channel Items -----------------")
    for i := 0; i < len(rssFeed.Channel.Item); i++ {
        rssFeed.Channel.Item[i].EscapeHtml()
        fmt.Printf("\t@@ Feed Item [%v]:\n", i + 1)
        fmt.Println("\t@@  Title: ", rssFeed.Channel.Item[i].Title)
        fmt.Println("\t@@  Publication Date : ", rssFeed.Channel.Item[i].PubDate)
        fmt.Println("\t@@  Link : ", rssFeed.Channel.Item[i].Link)
        fmt.Println("\t@@  Description : ", rssFeed.Channel.Item[i].Description)
    }

}

func aggHandler(s *state, user *database.User, c command) error {
    feeds, err := s.db.GetFeeds(context.Background())
    if err != nil {
        return fmt.Errorf("There is no feed to aggrigate")
    }

    for i := 0; i < len(feeds); i++ {
        feed, err := s.db.GetNextFeedToFetch(context.Background())
        if err != nil {
            break
        }

        res, err := fetchFeeds(context.Background(), feed.Url)
        if err != nil {
            return fmt.Errorf("Error: While aggrigating the blogs")
        }

        _, err = s.db.MarkFeedFetched(context.Background(), feed.ID)
        if err != nil {
            return err
        }

        prettyPrint(res)
    }

    return nil
}
func usersHandler(s *state, c command) error {
    users, err := s.db.GetUsers(context.Background())
    if err != nil {
        return fmt.Errorf("Error while selecting all users")
    }

    // now list all the users
    fmt.Printf("User Records\n")
    for _, user := range users {
        if user.Name == s.cfg.CurrentUserName {
            fmt.Printf("* %v (current)\n", user.Name)
            continue
        }
        fmt.Printf("* %v\n", user.Name)
    }

    return nil
}

func addFeedHandler(s *state, user *database.User, c command) error {
    if c.args == nil || len(c.args) < 2 {
        return fmt.Errorf("`addfeed` command expects a two arguments `name` `url`")
    }

    feedName, feedUrl := c.args[0], c.args[1]
    feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
        Name: feedName,
        UserID: user.ID,
        Url: feedUrl,
    })
    if err != nil {
        return fmt.Errorf("Error while creating the feed `%v`", err.Error())
    }

    s.db.CreateFeedFollows(context.Background(), database.CreateFeedFollowsParams{
        UserID: user.ID,
        FeedID: feed.ID,
    })

    fmt.Printf("Feed: successfully created: [%v]\n", feed)

    return nil
}

func unfollowFeedHandler(s *state, user *database.User, c command) error {
    if c.args == nil || len(c.args) < 1 {
        return fmt.Errorf("`follow` command expects one argument `url`")
    }

    url := c.args[0]

    feed, err := s.db.GetFeedByURL(context.Background(), url)
    if err != nil {
        return fmt.Errorf("The feed with this url doesn't exist")
    }

    err = s.db.DeleteFollowFeed(context.Background(), database.DeleteFollowFeedParams{
        UserID: user.ID,
        FeedID: feed.ID,
    })
    if err != nil {
        return fmt.Errorf("Error while deleting follow feed record")
    }

    fmt.Printf("Successfully Unfollowed Feed %v - %v\n", feed.Name, feed.Url)

    return nil
}

func followFeedHandler(s *state, user *database.User, c command) error {
    if c.args == nil || len(c.args) < 1 {
        return fmt.Errorf("`follow` command expects one argument `url`")
    }

    url := c.args[0]

    feed, err := s.db.GetFeedByURL(context.Background(), url)
    if err != nil {
        return fmt.Errorf("The feed with this url doesn't exist")
    }

    _, err = s.db.GetFeedByFeedIdAndUserId(context.Background(), database.GetFeedByFeedIdAndUserIdParams{
        UserID: user.ID,
        FeedID: feed.ID,
    })

    if err == nil { // if feed follow data exists
        return fmt.Errorf("You are already following this feed")
    }

    _, err = s.db.CreateFeedFollows(context.Background(), database.CreateFeedFollowsParams{
        UserID: user.ID,
        FeedID: feed.ID,
    })
    if err != nil {
        return fmt.Errorf("Error while creating follow feed record")
    }

    fmt.Printf("Successfully Followed %v - %v\n", feed.Name, feed.Url)

    return nil
}

func followingHandler(s *state, user *database.User, c command) error {
    ffs, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
    if err != nil {
        return fmt.Errorf("Error while selecting all the feeds")
    }

    i := 0
    for _, ff := range ffs {
        if i == 0 {
            fmt.Printf("user - %v follows:\n", user.Name)
        }
        fmt.Printf("Feed [%v]\n\tName: %v\n\tURL: %v\n\tFollowed Date: %v\n",
            i + 1, ff.Name, ff.Url, ff.CreatedAt.Time.Format("2006-01-02:15-04-05"),
            )
        i += 1
    }

    return nil
}

func loginHandler(s *state, c command) error {
    if c.args == nil || len(c.args) == 0 {
        return fmt.Errorf("`login` command expects a single argument the `username`")
    }

    // now lets select the user from the database
    user, err := s.db.GetUserByName(context.Background(), c.args[0])
    if err != nil {
        return fmt.Errorf("No user with the name `%v` is found", c.args[0])
    }

    if err := s.cfg.SetUser(user.Name); err != nil {
        return err
    }

    fmt.Printf("User has been set successfully to `%v`\n", user.Name)

    return nil
}

func main() {
    cfg, err := config.Read()
    if err != nil {
        fmt.Printf("Some error while reading the config file: `%v`\n", err.Error())
        os.Exit(1)
    }
    db, err := sql.Open("postgres", cfg.DbUrl)
    if err != nil {
        fmt.Printf("Some error while connecting to the database: `%v`\n", err.Error())
        os.Exit(1)
    }

    s := &state{
        db: database.New(db),
        cfg: cfg,
    }

    cmds := commands{
        cmds: make(map[string]commandHandler),
    }

    // all registerd commandsList
    cmds.register("agg", protectCommands(aggHandler))
    cmds.register("addfeed", protectCommands(addFeedHandler))
    cmds.register("follow", protectCommands(followFeedHandler))
    cmds.register("unfollow", protectCommands(unfollowFeedHandler))
    cmds.register("following", protectCommands(followingHandler))
    cmds.register("feeds", feedsHandler)
    cmds.register("users", usersHandler)
    cmds.register("login", loginHandler)
    cmds.register("reset", resetHandler)
    cmds.register("register", registerHandler)

    args := os.Args
    if len(args) < 2 {
        fmt.Printf("Error: no command provided\n")
        os.Exit(1)
    }

    cmd := command{
        name: args[1],
        args: args[2:],
    }

    err = cmds.run(s, cmd)
    if err != nil {
        fmt.Printf("Error: command `%v` failed due to %v\n", cmd.name, err.Error())
        os.Exit(1)
    }
}
