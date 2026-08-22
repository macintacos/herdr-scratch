package scratch

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

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
// release replaces it. Anything the user typed there is one upgrade away from
// being overwritten.
//
// The fields are not uniform, deliberately: Dismiss and NotifyAfter carry the
// defaults in DefaultConfig, while Width and Height are empty until the user
// sets one, because empty is how SizeArgs says "leave the size to herdr". So
// an empty Width means unset, but an empty Dismiss only ever appears on a
// Config nobody defaulted.
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
	return filepath.Join(dir, "herdr", "plugins", "config", PluginID, configFile)
}

const (
	// configFile is the name inside whichever config directory is resolved.
	configFile = "config.toml"
	// PluginID is the manifest's id — what herdr knows this plugin as, and the
	// directory it keeps its config under.
	PluginID = "user.scratch"
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
			// A key this does not recognise is a typo, and a typo that decodes
			// quietly is the failure this file exists to end: the user edits
			// `dismis`, nothing changes, and nothing says why.
			ErrorUnused: true,
			// A size may be written as a TOML integer — herdr's PopupSize takes
			// a cell count as readily as "70%" — so that one conversion is
			// allowed by name. WeaklyTypedInput would do it too, along with
			// turning `width = true` into a one-cell popup without complaint.
			DecodeHook: func(from, to reflect.Type, v any) (any, error) {
				if from.Kind() == reflect.Int64 && to.Kind() == reflect.String {
					return fmt.Sprint(v), nil
				}
				return v, nil
			},
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

// ManifestSettings reads the settings out of a manifest that is about to be
// replaced, so `link` can tell the user which config.toml lines carry them
// over.
//
// The manifest is where these lived before config.toml existed, and every
// release now writes a new one over the top of it — so the only chance to
// notice what was in there is the moment before it goes.
//
// A manifest this cannot read reports the zero Config rather than an error:
// nothing to migrate beats a wrong migration notice, since the file is being
// overwritten either way and a guessed line is one the user would paste.
func ManifestSettings(data []byte) Config {
	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(data), toml.Parser()); err != nil {
		return Config{}
	}

	// The pane herdr opens is the only place a manifest carries user settings:
	// the size on the entry itself, the dismiss chord inside the command it
	// runs. There is exactly one pane, so the first one is it.
	var panes []struct {
		Width   string   `koanf:"width"`
		Height  string   `koanf:"height"`
		Command []string `koanf:"command"`
	}
	if err := k.Unmarshal("panes", &panes); err != nil || len(panes) == 0 {
		return Config{}
	}

	cfg := Config{Width: panes[0].Width, Height: panes[0].Height}
	if m := dismissArg.FindStringSubmatch(strings.Join(panes[0].Command, " ")); m != nil {
		// Whichever quoting matched holds the chord; the other group is empty.
		cfg.Dismiss = m[1] + m[2]
	}
	return cfg
}

// DismissChord picks the chord to bind: the flag when it was given, then the
// config file, then the built-in default.
//
// flagGiven rather than a non-empty flag is what keeps the order honest. The
// flag has to lose to the config file in the ordinary case — otherwise a
// manifest that passes --dismiss would beat the file every time, which is the
// coupling moving these settings out exists to break — while a manifest
// somebody hand-edited before the move keeps working, chord and all.
//
// The caller reads it off its own flag set; taking the answer rather than the
// flag set is what keeps this side of the decision testable.
func DismissChord(flag string, flagGiven bool, cfg Config) string {
	if flagGiven {
		return flag
	}
	if cfg.Dismiss != "" {
		return cfg.Dismiss
	}
	return DefaultConfig().Dismiss
}

// SizeArgs spells whichever popup dimensions the user set as flags for
// `herdr plugin pane open`.
//
// Nothing at all when nothing is set: that leaves herdr applying the size the
// manifest declares, which is where the shipped default lives.
func SizeArgs(cfg Config) []string {
	var args []string
	if cfg.Width != "" {
		args = append(args, "--width", cfg.Width)
	}
	if cfg.Height != "" {
		args = append(args, "--height", cfg.Height)
	}
	return args
}

// MigratedSettings names the settings a manifest being replaced carried that
// would not survive it, spelled as the config.toml lines that carry them over.
//
// Each is measured against what will be in force once the shipped manifest is
// in place: that manifest's own size, and — since it names no chord at all —
// the built-in default chord. Measuring the chord against the shipped manifest
// instead would report every untouched install, whose "C-b '" is the same
// chord it is about to get anyway.
//
// Only a value the user actually typed counts: an empty one is a setting that
// manifest never had, not one to move.
func MigratedSettings(had, shipped Config) []string {
	var lines []string
	for _, setting := range []struct{ key, was, now string }{
		{"dismiss", had.Dismiss, DefaultConfig().Dismiss},
		{"width", had.Width, shipped.Width},
		{"height", had.Height, shipped.Height},
	} {
		if setting.was != "" && setting.was != setting.now {
			lines = append(lines, fmt.Sprintf("%s = %q", setting.key, setting.was))
		}
	}
	return lines
}

// dismissArg picks the chord out of the shell string a pane command runs.
//
// A chord is two keys with a space between them, so the line has to quote it —
// either way round, because a hand-edited manifest is exactly what this reads.
var dismissArg = regexp.MustCompile(`--dismiss[= ]+(?:"([^"]*)"|'([^']*)')`)
