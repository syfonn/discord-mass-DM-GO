// Copyright (C) 2021 github.com/V4NSH4J
//
// This source code has been released under the GNU Affero General Public
// License v3.0. A copy of this license is available at
// https://www.gnu.org/licenses/agpl-3.0.en.html

package instance

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/V4NSH4J/discord-mass-dm-GO/utilities"
	"github.com/gorilla/websocket"
)

type Instance struct {
	Token           string
	Email           string
	Password        string
	Proxy           string
	Cookie          string
	Fingerprint     string
	Messages        []Message
	Count           int
	LastQuery       string
	LastCount       int
	Members         []User
	AllMembers      []User
	Retry           int
	ScrapeCount     int
	ID              string
	Receiver        bool
	Config          Config
	GatewayProxy    string
	Client          *http.Client
	WG              *sync.WaitGroup
	Ws              *Connection
	fatal           chan error
	Invited         bool
	TimeServerCheck time.Time
	ChangedName     bool
	ChangedAvatar   bool
	LastID          int
	LastIDstr       string
	Mode            int
	UserAgent       string
	XSuper          string
	Reacted         []ReactInfo
	ReactChannel    chan (ReactInfo)
	MessageNumber   int
	Locale          string
}

func (in *Instance) StartWS() error {
	ws, err := in.NewConnection(in.wsFatalHandler)
	if err != nil {
		return fmt.Errorf("failed to create websocket connection: %s", err)
	}
	in.Ws = ws
	return nil
}

func (in *Instance) wsFatalHandler(err error) {
	if closeErr, ok := err.(*websocket.CloseError); ok && closeErr.Code == 4004 {
		in.fatal <- fmt.Errorf("websocket closed: authentication failed, try using a new token")
		return
	}
	if strings.Contains(err.Error(), "4004") {
		utilities.LogLocked("Error while opening websocket, Authentication failed %v", in.Token)
		return
	}
	utilities.LogSuccess("Websocket closed %v %v", err, in.Token)
	in.Receiver = false
	in.Ws, err = in.NewConnection(in.wsFatalHandler)
	if err != nil {
		in.fatal <- fmt.Errorf("failed to create websocket connection: %s", err)
		return
	}
	utilities.LogSuccess("Reconnected To Websocket %v", in.Token)
}

func GetEverything() (Config, []Instance, error) {
	var cfg Config
	var instances []Instance
	var err error
	var tokens []string
	var proxies []string
	var proxy string
	var xsuper string
	var ua string

	// Load config
	cfg, err = GetConfig()
	if err != nil {
		return cfg, instances, err
	}
	supportedProtocols := []string{"http", "https", "socks4", "socks5"}
	if cfg.ProxySettings.ProxyProtocol != "" && !utilities.Contains(supportedProtocols, cfg.ProxySettings.ProxyProtocol) {
		utilities.LogErr(" You're using an unsupported proxy protocol. Assuming http by default")
		cfg.ProxySettings.ProxyProtocol = "http"
	}
	if cfg.ProxySettings.ProxyProtocol == "https" {
		cfg.ProxySettings.ProxyProtocol = "http"
	}
	if cfg.CaptchaSettings.CaptchaAPI == "" {
		utilities.LogErr(" You're not using a Captcha API, some functionality like invite joining might be unavailable")
	}
	if cfg.ProxySettings.Proxy != "" && os.Getenv("HTTPS_PROXY") == "" {
		os.Setenv("HTTPS_PROXY", cfg.ProxySettings.ProxyProtocol+"://"+cfg.ProxySettings.Proxy)
	}
	if !cfg.ProxySettings.ProxyFromFile && cfg.ProxySettings.ProxyForCaptcha {
		utilities.LogErr(" You must enabe proxy_from_file to use proxy_for_captcha")
		cfg.ProxySettings.ProxyForCaptcha = false
	}
	cfg.OtherSettings.Mode = 0
	locales := []string{"de-AT", "de-DE", "de-IT", "de-LI", "de-LU", "en-AG", "en-AI", "en-AT", "en-AU", "en-BB", "en-CA", "en-BS", "en-CH", "en-DE", "en-FI", "en-GB", "en-HK", "en-IN", "en-MY", "en-SG", "en-US", "fr-CA", "fr-FR"}
	locale := locales[rand.Intn(len(locales))]
	if cfg.OtherSettings.Mode == 1 {
		// Discord App
		ua, xsuper = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) discord/0.0.267 Chrome/91.0.4472.164 Electron/13.6.6 Safari/537.36", "eyJvcyI6Ik1hYyBPUyBYIiwiYnJvd3NlciI6IkRpc2NvcmQgQ2xpZW50IiwicmVsZWFzZV9jaGFubmVsIjoic3RhYmxlIiwiY2xpZW50X3ZlcnNpb24iOiIwLjAuMjY3Iiwib3NfdmVyc2lvbiI6IjIxLjUuMCIsIm9zX2FyY2giOiJhcm02NCIsInN5c3RlbV9sb2NhbGUiOiJlbi1VUyIsImNsaWVudF9idWlsZF9udW1iZXIiOjEzNTQwMiwiY2xpZW50X2V2ZW50X3NvdXJjZSI6bnVsbH0="
		// } else if cfg.OtherSettings.Mode == 2 {
		// 	// Mobile disabled for now, as too lazy to proxy requests
		// 	ua, xsuper = "Discord/32249 CFNetwork/1331.0.7 Darwin/21.4.0", "eyJvcyI6ImlPUyIsImJyb3dzZXIiOiJEaXNjb3JkIGlPUyIsImRldmljZSI6ImlQYWQxMywxNiIsInN5c3RlbV9sb2NhbGUiOiJlbi1JTiIsImNsaWVudF92ZXJzaW9uIjoiMTI0LjAiLCJyZWxlYXNlX2NoYW5uZWwiOiJzdGFibGUiLCJkZXZpY2VfYWR2ZXJ0aXNlcl9pZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMCIsImRldmljZV92ZW5kb3JfaWQiOiJBMTgzNkNFRC1BRDI5LTRGRTAtQjVDNC0zODQ0NDU0MEFFQTciLCJicm93c2VyX3VzZXJfYWdlbnQiOiIiLCJicm93c2VyX3ZlcnNpb24iOiIiLCJvc192ZXJzaW9uIjoiMTUuNC4xIiwiY2xpZW50X2J1aWxkX251bWJlciI6MzIyNDcsImNsaWVudF9ldmVudF9zb3VyY2UiOm51bGx9"
	} else {
		// Browser
		ua, xsuper = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36", XSuper(locale)
		//
	}

	// Load instances
	tokens, err = utilities.ReadLines("tokens.txt")
	if err != nil {
		return cfg, instances, err
	}
	if len(tokens) == 0 {
		return cfg, instances, fmt.Errorf("no tokens found in tokens.txt")
	}

	if cfg.ProxySettings.ProxyFromFile {
		proxies, err = utilities.ReadLines("proxies.txt")
		if err != nil {
			return cfg, instances, err
		}
		if len(proxies) == 0 {
			return cfg, instances, fmt.Errorf("no proxies found in proxies.txt")
		}
	}
	var Gproxy string
	var instanceToken string
	var email string
	var password string
	reg := regexp.MustCompile(`(.+):(.+):(.+)`)
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "" {
			continue
		}
		if reg.MatchString(tokens[i]) {
			parts := strings.Split(tokens[i], ":")
			instanceToken = parts[2]
			email = parts[0]
			password = parts[1]
		} else {
			instanceToken = tokens[i]
		}
		if cfg.ProxySettings.ProxyFromFile {
			proxy = proxies[rand.Intn(len(proxies))]
			Gproxy = proxy
		} else {
			proxy = ""
		}
		client, err := InitClient(proxy, cfg)
		if err != nil {
			return cfg, instances, fmt.Errorf("couldn't initialize client: %v", err)
		}
		// proxy is put in struct only to be used by gateway. If proxy for gateway is disabled, it will be empty
		if !cfg.ProxySettings.GatewayProxy {
			Gproxy = ""
		}
		instances = append(instances, Instance{Client: client, Token: instanceToken, Proxy: proxy, Config: cfg, GatewayProxy: Gproxy, Email: email, Password: password, UserAgent: ua, XSuper: xsuper, Locale: locale})
	}
	if len(instances) == 0 {
		utilities.LogErr(" You may be using 0 tokens")
	}

	return cfg, instances, nil

}

func SetMessages(instances []Instance, messages []Message) error {
	var err error
	if len(messages) == 0 {
		messages, err = GetMessage()
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return fmt.Errorf("no messages found in messages.txt")
		}
		for i := 0; i < len(instances); i++ {
			instances[i].Messages = messages
		}
	} else {
		for i := 0; i < len(instances); i++ {
			instances[i].Messages = messages
		}
	}

	return nil
}

func (in *Instance) CensorToken() string {
	if len(in.Token) == 0 {
		return ""
	}
	if in.Config.OtherSettings.CensorToken {
		var censored string
		l := len(in.Token)
		uncensoredPart := int(2 * l / 3)
		for i := 0; i < l; i++ {
			if i < uncensoredPart {
				censored += string(in.Token[i])
			} else {
				censored += "*"
			}
		}
		return censored
	} else {
		return in.Token
	}

}

func (in *Instance) WriteInstanceToFile(path string) {
	var line string
	if in.Email != "" && in.Password != "" {
		line = fmt.Sprintf("%s:%s:%s", in.Email, in.Password, in.Token)
	} else {
		line = in.Token
	}
	_ = utilities.WriteLinesPath(path, line)
}

type Head struct {
	XSuperProperties string `json:"x-super-properties"`
	Useragent        string `json:"useragent"`
}

func XSuper(locale string) string {
	dec := fmt.Sprintf(`{"os":"Mac OS X","browser":"Chrome","device":"","system_locale":"%s","browser_user_agent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36","browser_version":"103.0.5060.114","os_version":"10.15.7","referrer":"","referring_domain":"","referrer_current":"","referring_domain_current":"","release_channel":"stable","client_build_number":%d,"client_event_source":null}`, locale ,rand.Intn(100000) + rand.Intn(10000)+ rand.Intn(1000) + rand.Intn(100))
	return base64.StdEncoding.EncodeToString([]byte(dec))
	
}