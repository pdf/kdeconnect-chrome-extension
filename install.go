package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/kardianos/osext"
	"github.com/paulrademacher/climenu"
)

const (
	def                = `default`
	appName            = `com.0xc0dedbad.kdeconnect_chrome`
	defaultExtensionID = `ofmplbbfigookafjahpeepbggpofdhbo`
)

var (
	manifestTemplate = template.Must(template.New(`manifest`).Parse(`{
  "name": "com.0xc0dedbad.kdeconnect_chrome",
  "description": "KDE Connect",
  "path": "{{.Path}}",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://{{.ExtensionID}}/"
  ]
}`))
	manifestFirefoxTemplate = template.Must(template.New(`manifest`).Parse(`{
  "name": "com.0xc0dedbad.kdeconnect_chrome",
  "description": "KDE Connect",
  "path": "{{.Path}}",
  "type": "stdio",
  "allowed_extensions": [
	  "kde-connect@0xc0dedbad.com"
  ]
}`))

	// OS/browser/user/path
	installMappings map[string]map[string]map[string]string
)

type manifest struct {
	Path        string
	ExtensionID string
}

func doInstall(path, browser, extensionID string) error {
	daemonPath := filepath.Join(path, appName)
	templatePath := filepath.Join(path, fmt.Sprintf("%s.json", appName))
	var err error

	if err = os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	exe, err := osext.Executable()
	if err != nil {
		return err
	}
	in, err := os.Open(exe)
	defer func() {
		if e := in.Close(); err == nil && e != nil {
			fmt.Println(e)
			panic(e)
		}
	}()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(daemonPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer func() {
		if e := out.Close(); err == nil && e != nil {
			fmt.Println(e)
			panic(e)
		}
	}()
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	man, err := os.OpenFile(templatePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() {
		if e := man.Close(); err != nil {
			fmt.Println(e)
			panic(e)
		}
	}()
	if err != nil {
		return err
	}
	if browser == `firefox` {
		err = manifestFirefoxTemplate.Execute(man, manifest{
			Path: daemonPath,
		})
		return err
	}
	err = manifestTemplate.Execute(man, manifest{
		Path:        daemonPath,
		ExtensionID: extensionID,
	})
	return err
}

func hasCustom(selection []string) bool {
	for _, s := range selection {
		if s == `custom` {
			return true
		}
	}
	return false
}

func install(developer bool) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	username := u.Username
	operatingSystem := runtime.GOOS

	switch username {
	case `root`:
	default:
		username = def
	}

	switch operatingSystem {
	case `darwin`:
	default:
		operatingSystem = def
	}

	xdgConfigHome := os.Getenv(`XDG_CONFIG_HOME`)
	if xdgConfigHome == `` {
		xdgConfigHome = filepath.Join(u.HomeDir, `.config`)
	}

	installMappings = map[string]map[string]map[string]string{
		def: {
			def: {
				def: filepath.Join(
					xdgConfigHome, `google-chrome`, `NativeMessagingHosts`,
				),
				`root`: `/etc/opt/chrome/native-messaging-hosts`,
			},
			`vivaldi`: {
				def: filepath.Join(
					xdgConfigHome, `vivaldi`, `NativeMessagingHosts`,
				),
				`root`: `/etc/vivaldi/native-messaging-hosts`,
			},
			`brave`: {
				def: filepath.Join(
					xdgConfigHome, `BraveSoftware`, `Brave-Browser`, `NativeMessagingHosts`,
				),
				`root`: `/etc/chromium/native-messaging-hosts`,
			},
			`chromium`: {
				def: filepath.Join(
					xdgConfigHome, `chromium`, `NativeMessagingHosts`,
				),
				`root`: `/etc/chromium/native-messaging-hosts`,
			},
			`firefox`: {
				def: filepath.Join(
					u.HomeDir, `.mozilla`, `native-messaging-hosts`,
				),
				`root`: `/usr/lib/mozilla/native-messaging-hosts`,
			},
		},
		`darwin`: {
			def: {
				def: filepath.Join(
					u.HomeDir, `Library`, `Application Support`, `Google`, `Chrome`, `NativeMessagingHosts`,
				),
				`root`: `/Library/Google/Chrome/NativeMessagingHosts`,
			},
			`vivaldi`: {
				def: filepath.Join(
					u.HomeDir, `Library`, `Application Support`, `Vivaldi`, `NativeMessagingHosts`,
				),
				`root`: `/Library/Vivaldi/NativeMessagingHosts`,
			},
			`brave`: {
				def: filepath.Join(
					u.HomeDir, `Library`, `Application Support`, `Chromium`, `NativeMessagingHosts`,
				),
				`root`: `/Library/Application Support/Chromium/NativeMessagingHosts`,
			},
			`chromium`: {
				def: filepath.Join(
					u.HomeDir, `Library`, `Application Support`, `Chromium`, `NativeMessagingHosts`,
				),
				`root`: `/Library/Application Support/Chromium/NativeMessagingHosts`,
			},
			`firefox`: {
				def: filepath.Join(
					u.HomeDir, `Library`, `Application Support`, `Mozilla`, `NativeMessagingHosts`,
				),
				`root`: `/Library/Application Support/Mozilla/NativeMessagingHosts`,
			},
		},
	}

	menu := climenu.NewCheckboxMenu(`Browser Selection`, `Select browser(s) for native host installation (Space to select, Enter to confirm)`, `OK`, `Cancel`)
	menu.AddMenuItem(`Chrome/Opera`, def)
	menu.AddMenuItem(`Chromium`, `chromium`)
	menu.AddMenuItem(`Brave`, `brave`)
	menu.AddMenuItem(`Vivaldi`, `vivaldi`)
	menu.AddMenuItem(`Firefox`, `firefox`)
	menu.AddMenuItem(`Custom`, `custom`)

	var (
		selection = make([]string, 0)
		escaped   bool
	)

	for len(selection) == 0 {
		selection, escaped = menu.Run()
		if escaped {
			os.Exit(1)
		}
	}

	if hasCustom(selection) {
		var response string
		for response == `` {
			defaultPath := installMappings[operatingSystem][username][def]
			response = climenu.GetText(`Enter the destination native messaging hosts path`, defaultPath)
		}
		selection = append(selection, response)
	}

	var extensionID string
	if developer {
		for extensionID == `` {
			extensionID = climenu.GetText(`Extension ID (Enter accepts default)`, defaultExtensionID)
		}
	} else {
		extensionID = defaultExtensionID
	}

	for _, s := range selection {
		if s == `custom` {
			continue
		}
		path, ok := installMappings[operatingSystem][s][username]
		if !ok {
			// custom path
			path = s
		}
		if err := doInstall(path, s, extensionID); err != nil {
			fmt.Println(`Failed installing the binary, ensure that your browser(s) are closed and that you have the required permissions.`)
			return err
		}
	}

	fmt.Println(`Done.`)
	return nil
}
