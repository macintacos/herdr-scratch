package scratch

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

// Config is every setting the user owns.
//
// It is a file of its own rather than a section of the manifest because the
// manifest is the plugin's: herdr resolves it, the package ships it, and every
// release replaces it. Anything the user typed in there was one upgrade away
// from being overwritten — or, while `link` refused to overwrite it, one
// upgrade away from never arriving.
type Config struct {
	Dismiss     string `koanf:"dismiss"      validate:"omitempty,dismisschord"`
	NotifyAfter int    `koanf:"notify_after" validate:"gte=0"`
	Width       string `koanf:"width"        validate:"omitempty,popupsize"`
	Height      string `koanf:"height"       validate:"omitempty,popupsize"`
}

// DefaultConfig is what every fallback path returns: no file, an unreadable
// one, or one that does not validate.
//
// Width and Height are empty rather than "70%" on purpose. Size already has a
// shipped default in the manifest, which herdr applies to an open request that
// names no size — so empty here means "leave it to herdr" and keeps one place,
// not two, deciding how big the popup is.
func DefaultConfig() Config {
	return Config{Dismiss: "C-b '", NotifyAfter: 10000}
}

// ConfigPath is the file the user edits.
//
// herdr sets HERDR_PLUGIN_CONFIG_DIR on every plugin command, so the popup and
// the toggle never have to work the path out. The two XDG tiers below it are
// for the invocations herdr is not making — `link`, and a binary run by hand —
// which still have to name the same file.
//
// lookup is the environment reader and home the fallback base, both injected so
// this stays testable.
func ConfigPath(lookup func(string) string, home string) string {
	if dir := lookup("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, configFile)
	}
	dir := lookup("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "herdr", "plugins", "config", pluginID, configFile)
}

const (
	// configFile is the name inside whichever config directory is resolved.
	configFile = "config.toml"
	// pluginID is the manifest's id, which is also the directory herdr keeps
	// this plugin's config under.
	pluginID = "user.scratch"
)

// LoadConfig reads the settings out of the bytes of config.toml.
//
// Taking bytes rather than a path keeps this pure and leaves the read to the
// caller, matching the rest of this package. It also makes the missing-file
// case free: an empty read is a config that sets nothing.
//
// Every failure returns DefaultConfig beside the error, so a caller with
// nowhere to report one — these run from keypresses — can ignore it and still
// hold a Config it can act on. Keys the file omits keep their defaults rather
// than zeroing, because setting one thing must not unset the others.
func LoadConfig(data []byte) (Config, error) {
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(data), toml.Parser()); err != nil {
		return DefaultConfig(), fmt.Errorf("%s is not valid TOML: %w", configFile, err)
	}

	cfg := DefaultConfig()
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			// So `width = 80` and `width = "70%"` both land in the same string
			// field — herdr's PopupSize accepts either spelling.
			WeaklyTypedInput: true,
			TagName:          "koanf",
			Result:           &cfg,
		},
	}); err != nil {
		return DefaultConfig(), fmt.Errorf("%s: %w", configFile, err)
	}

	if err := configValidator.Struct(cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("%s: %w", configFile, err)
	}
	return cfg, nil
}

// configValidator holds the two rules that belong to this plugin rather than to
// the library: a chord tmux can bind, and a size herdr will accept.
var configValidator = newConfigValidator()

func newConfigValidator() *validator.Validate {
	v := validator.New()
	// Name the key the user typed, not the Go field it decoded into.
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("koanf")
	})
	_ = v.RegisterValidation("dismisschord", func(fl validator.FieldLevel) bool {
		_, _, ok := DismissKeys(fl.Field().String())
		return ok
	})
	_ = v.RegisterValidation("popupsize", func(fl validator.FieldLevel) bool {
		return validPopupSize(fl.Field().String())
	})
	return v
}

// popupPercent is herdr's own PopupSize pattern: 1% through 100%.
var popupPercent = regexp.MustCompile(`^(100|[1-9][0-9]?)%$`)

// validPopupSize reports whether herdr would take this as a popup dimension —
// a percentage of the terminal, or a count of cells.
//
// Checked here so a bad size is a line in the log naming the file, rather than
// a pane open herdr refuses for reasons that never reach the user.
func validPopupSize(size string) bool {
	if popupPercent.MatchString(size) {
		return true
	}
	cells, err := strconv.Atoi(size)
	return err == nil && cells >= 0 && cells <= 65535
}
