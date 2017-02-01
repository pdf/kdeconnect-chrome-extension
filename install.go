package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

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

	// OS/browser/user/path
	installMappings map[string]map[string]map[string]string
)

type manifest struct {
	Path        string
	ExtensionID string
}

func doInstall(path, extensionID string) error {
	daemonPath := filepath.Join(path, appName)
	templatePath := filepath.Join(path, fmt.Sprintf("%s.json", appName))

	if err := os.MkdirAll(path, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	in, err := os.Open(os.Args[0])
	defer func() {
		if e := in.Close(); err != nil {
			fmt.Println(e)
			panic(e)
		}
	}()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(daemonPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer func() {
		if e := out.Close(); err != nil {
			fmt.Println(e)
			panic(e)
		}
	}()
	if err != nil {
		return err
	}
	//fmt.Println(`Copying daemon`, daemonPath)
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
	//fmt.Println(`Writing template`, templatePath)
	if err = manifestTemplate.Execute(man, manifest{
		Path:        daemonPath,
		ExtensionID: extensionID,
	}); err != nil {
		return err
	}

	return nil
}

func hasCustom(selection []string) bool {
	for _, s := range selection {
		if s == `custom` {
			return true
		}
	}
	return false
}

func install() error {
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

	installMappings = map[string]map[string]map[string]string{
		def: {
			def: {
				def: filepath.Join(
					u.HomeDir, `/.config/google-chrome/NativeMessagingHosts`,
				),
				`root`: `/etc/opt/chrome/native-messaging-hosts`,
			},
			`vivaldi`: {
				def: filepath.Join(
					u.HomeDir, `/.config/vivaldi/NativeMessagingHosts`,
				),
				`root`: `/etc/vivaldi/native-messaging-hosts`,
			},
			`chromium`: {
				def: filepath.Join(
					u.HomeDir, `/.config/chromium/NativeMessagingHosts`,
				),
				`root`: `/etc/chromium/native-messaging-hosts`,
			},
		},
		`darwin`: {
			def: {
				def: filepath.Join(
					u.HomeDir, `/Library/Application Support/Google/Chrome/NativeMessagingHosts`,
				),
				`root`: `/Library/Google/Chrome/NativeMessagingHosts`,
			},
			`vivaldi`: {
				def: filepath.Join(
					u.HomeDir, `/Library/Application Support/Vivaldi/NativeMessagingHosts`,
				),
				`root`: `/Library/Vivaldi/NativeMessagingHosts`,
			},
			`chromium`: {
				def: filepath.Join(
					u.HomeDir, `/Library/Application Support/Chromium/NativeMessagingHosts`,
				),
				`root`: `/Library/Application Support/Chromium/NativeMessagingHosts`,
			},
		},
	}

	menu := climenu.NewCheckboxMenu(`Browser Selection`, `Select browser(s) for native host installation`, `OK`, `Cancel`)
	menu.AddMenuItem(`Chrome/Opera`, def)
	menu.AddMenuItem(`Chromium`, `chromium`)
	menu.AddMenuItem(`Vivaldi`, `vivaldi`)
	menu.AddMenuItem(`Custom`, `custom`)

	selection, escaped := menu.Run()
	if escaped {
		os.Exit(1)
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
	for extensionID == `` {
		extensionID = climenu.GetText(`Extension ID (Enter accepts default)`, defaultExtensionID)
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
		if err := doInstall(path, extensionID); err != nil {
			return err
		}
	}

	fmt.Println(`Done.`)
	return nil
}
