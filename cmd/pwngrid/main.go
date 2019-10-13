package main

import (
	"flag"
	"fmt"
	"github.com/evilsocket/islazy/fs"
	"github.com/evilsocket/islazy/log"
	"github.com/evilsocket/pwngrid/api"
	"github.com/evilsocket/pwngrid/crypto"
	"github.com/evilsocket/pwngrid/mesh"
	"github.com/evilsocket/pwngrid/models"
	"github.com/evilsocket/pwngrid/utils"
	"github.com/evilsocket/pwngrid/version"
	"github.com/joho/godotenv"
	"time"
)

var (
	debug    = false
	routes   = false
	ver      = false
	wait     = false
	inbox    = false
	del      = false
	unread   = false
	clear    = false
	receiver = ""
	message  = ""
	output   = ""
	page     = 1
	id       = 0
	address  = "0.0.0.0:8666"
	env      = ".env"
	iface    = "mon0"
	keysPath = ""
	keys     = (*crypto.KeyPair)(nil)
	peer     = (*mesh.Peer)(nil)
)

func init() {
	flag.BoolVar(&ver, "version", ver, "Print version and exit.")
	flag.BoolVar(&debug, "debug", debug, "Enable debug logs.")
	flag.BoolVar(&routes, "routes", routes, "Generate routes documentation.")
	flag.StringVar(&log.Output, "log", log.Output, "Log file path or empty for standard output.")
	flag.StringVar(&address, "address", address, "API address.")
	flag.StringVar(&env, "env", env, "Load .env from.")

	flag.StringVar(&keysPath, "keys", keysPath, "If set, will load RSA keys from this folder and start in peer mode.")
	flag.BoolVar(&wait, "wait", wait, "Wait for keys to be generated.")
	flag.IntVar(&api.ClientTimeout, "client-timeout", api.ClientTimeout, "Timeout in seconds for requests to the server when in peer mode.")
	flag.StringVar(&api.ClientTokenFile, "client-token", api.ClientTokenFile, "File where to store the API token.")

	flag.StringVar(&iface, "iface", iface, "Monitor interface to use for mesh advertising.")
	flag.IntVar(&mesh.SignalingPeriod, "signaling-period", mesh.SignalingPeriod, "Period in milliseconds for mesh signaling frames.")

	flag.BoolVar(&inbox, "inbox", inbox, "Show inbox.")
	flag.StringVar(&receiver, "send", receiver, "Receiver unit fingerprint.")
	flag.StringVar(&message, "message", message, "Message body or file path if prefixed by @.")
	flag.StringVar(&output, "output", output, "Write message body to this file instead of the standard output.")
	flag.BoolVar(&del, "delete", del, "Delete the specified message.")
	flag.BoolVar(&unread, "unread", unread, "Unread the specified message.")
	flag.BoolVar(&clear, "clear", unread, "Delete all messages of the given page of the inbox.")
	flag.IntVar(&page, "page", page, "Inbox page.")
	flag.IntVar(&id, "id", id, "Message id.")
}

func main() {
	var err error

	flag.Parse()

	if ver {
		fmt.Println(version.Version)
		return
	}

	if debug {
		log.Level = log.DEBUG
	} else {
		log.Level = log.INFO
	}
	log.OnFatal = log.ExitOnFatal

	if err := log.Open(); err != nil {
		panic(err)
	}
	defer log.Close()

	mode := "server"
	if keysPath != "" {
		mode = "peer"
	}

	if (inbox || receiver != "") && keysPath == "" {
		keysPath = "/etc/pwnagotchi/"
	}

	log.Info("pwngrid v%s starting in %s mode ...", version.Version, mode)

	if mode == "peer" {
		mode = "peer"

		if wait {
			// wait for keys to be generated
			privPath := crypto.PrivatePath(keysPath)
			for {
				if !fs.Exists(privPath) {
					log.Debug("waiting for %s ...", privPath)
					time.Sleep(1 * time.Second)
				} else {
					// give it a moment to finish disk sync
					time.Sleep(2 * time.Second)
					log.Info("%s found", privPath)
					break
				}
			}
		}

		if keys, err = crypto.Load(keysPath); err != nil {
			log.Fatal("error while loading keys from %s: %v", keysPath, err)
		}

		peer = mesh.MakeLocalPeer(utils.Hostname(), keys)
		if err = peer.StartAdvertising(iface); err != nil {
			log.Fatal("error while starting signaling: %v", err)
		}
	}

	if keys == nil {
		if err := godotenv.Load(env); err != nil {
			log.Fatal("%v", err)
		}

		if err := models.Setup(); err != nil {
			log.Fatal("%v", err)
		}
	}

	err, server := api.Setup(keys, peer, routes)
	if err != nil {
		log.Fatal("%v", err)
	}

	if keys != nil {
		doInbox(server)
	}

	server.Run(address)
}
