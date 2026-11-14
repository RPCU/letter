// Package config reads the application settings from LETTER_* environment
// variables.
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The defaults make an apology letter. Every one can be replaced, which is
// how the same page wishes someone a good trip instead.
const (
	// DefaultName is who the letter is for when LETTER_NAME is unset.
	DefaultName = "you"
	// DefaultTitle heads the letter when LETTER_TITLE is unset.
	DefaultTitle = "I’m sorry"
	// DefaultQuestion is asked under the letter when LETTER_QUESTION is unset.
	DefaultQuestion = "Do you forgive me?"
	// DefaultYes is the yes button when LETTER_YES is unset.
	DefaultYes = "Yes"
	// DefaultYesTitle heads the card shown once they said yes.
	DefaultYesTitle = "Thank you"
	// DefaultStamp is pressed on that card.
	DefaultStamp = "forgiven"
	// DefaultSeal is on the envelope's wax seal.
	DefaultSeal = "🥺"
	// DefaultTheme is the page's palette.
	DefaultTheme = "rose"
	// DefaultGiftIntro announces LETTER_GIFT on the yes card.
	DefaultGiftIntro = "And a little something, to make it up to you"
	// DefaultModel is the Claude model the notes are written with.
	DefaultModel = "sonnet"
	// DefaultCommand is the Claude Code CLI.
	DefaultCommand = "claude"
	// DefaultYesStyle is how Claude answers a yes.
	DefaultYesStyle = "a warm, relieved thank-you of one or two sentences, then exactly three small, concrete and sincere promises, " +
		"each on its own line starting with \"— \""
)

// DefaultMessage is the letter itself when LETTER_MESSAGE is unset: blank
// lines separate paragraphs.
const DefaultMessage = `I’ve been thinking about what happened, and you deserve more than a quick text.

I’m sorry. Not the kind of sorry that just wants this to be over, but the kind that knows I let you down, and wishes I hadn’t.

You matter to me, and I’d like to do better. Starting now.`

// DefaultNoLabels are the no button's words, one more each time they say no.
// Once they run out the no button slips away, leaving only yes.
var DefaultNoLabels = []string{"No", "Still no", "Are you sure?", "Really sure?", "Think it over?", "Last chance…"}

// DefaultNoStyles are the forms Claude's note can take after a no: each
// visit draws them in a new order (LETTER_SHUFFLE). {name} becomes
// LETTER_NAME.
var DefaultNoStyles = []string{
	"a short, sincere note: no jokes, just honesty",
	"a haiku (three lines, 5-7-5), tender and a little funny",
	"a mock-formal petition to the Court of {name}’s Heart, tongue-in-cheek, in three short clauses",
	"a weather forecast of your remorse, light and charming",
	"a recipe for forgiveness: a title line, then three tiny ingredients, one per line",
	"a telegram: clipped phrases, each ending in STOP",
	"a lost-and-found notice for something you lost through this (their good mood, your dignity…), playful",
	"a museum placard presenting your apology as an exhibit: a title line, a medium line, one line of curator’s note",
	"the terms and conditions of your apology: three tiny numbered clauses, the fine print sweet",
	"a ship captain’s log entry, wry and a little lost at sea",
	"today’s horoscope for {name}, all about forgiveness, gently mischievous",
	"a two-line play: a stage direction in brackets, then one line of dialogue",
	"a menu of apologies: three dishes, one per line, each with a tiny description",
	"a review of your own apology, with a star rating, self-deprecating",
	"a postcard from the doghouse",
	"a crossword clue whose answer is SORRY or FORGIVE, with the letter count, then a one-line hint",
	"a plain, quiet admission of one thing you would do differently",
	"a limerick, clever rather than silly",
	"the first lines of a bedtime story, starting “Once upon a time”, that turn into an apology",
	"a newspaper headline, then a one-sentence lede about your remorse",
}

// DefaultNoLastStyle is the note's form after the last no, as the no
// button leaves.
const DefaultNoLastStyle = "a gracious note: you respect their answer and you’ll be right here, then, lightly, that the other button is still waiting"

// DefaultImages are small, concrete images, one of which Claude is asked to
// weave into each note so no two visits sound alike.
var DefaultImages = []string{
	"a paper boat", "an umbrella", "a cup of tea going cold", "a lighthouse", "a houseplant", "odd socks", "a bicycle bell",
	"an overdue library book", "a jar of honey", "a candle", "a train ticket", "a kite", "a lemon", "a snail", "a postage stamp",
	"the moon", "a toaster", "a cactus", "a comet", "an origami crane", "a scarf", "a sandcastle", "an empty bench", "a pigeon",
	"a paper lantern", "a cat on a windowsill", "a piano", "a folded map", "a sunflower", "a snow globe", "a teaspoon",
	"a mixtape", "a crossword", "a croissant", "a rubber duck", "a compass",
}

// DefaultReasons are the chips offered after a no ("Why not?"): Claude
// answers the one they tap.
var DefaultReasons = []string{"Too soon", "Still a bit upset", "Just teasing you", "Convince me"}

// DefaultFallbackWhy answers each reason in turn when Claude can't write;
// the last one stays.
var DefaultFallbackWhy = []string{
	"Fair. Take all the time you need, I’m not going anywhere.",
	"That’s allowed. I’d rather you be honestly upset than politely fine.",
	"I suspected as much. Your comic timing was always better than mine.",
	"Challenge accepted. Exhibit A: I made you a whole little website.",
}

// DefaultFallbackOffers are the canned offers when Claude can't write, three
// per round.
var DefaultFallbackOffers = []string{
	"Your favourite dinner, my treat, my reservation.",
	"A whole evening where my phone stays in a drawer.",
	"A handwritten letter, on actual paper.",
	"Breakfast in bed, croissants included.",
	"I plan the next weekend, start to finish.",
	"A playlist made just for you, liner notes and all.",
	"A picnic with everything you like and nothing you don’t.",
	"One “you were right” coupon, redeemable forever.",
	"Dinner, flowers, and I do the dishes for a week.",
}

// DefaultFallbackDeal seals a deal when Claude can't write.
const DefaultFallbackDeal = "Deal. Consider it signed, sealed, and already on my calendar."

// DefaultFallbackCloud are the cloud's asides when Claude can't write.
var DefaultFallbackCloud = []string{
	"I’m just here for the drama.",
	"Don’t look at me, I’m only weather.",
	"Honestly, I could go either way.",
	"For the record, the sun is rooting for this.",
	"I’ll stay as long as it takes. I have nowhere to be.",
	"Clouds don’t take sides. Mostly.",
}

// DefaultFallbackNo is what the page says after each no, in turn, when Claude
// can't write (not signed in, usage limits…). {name} becomes LETTER_NAME.
var DefaultFallbackNo = []string{
	"That’s fair. I’d say no to me too, a little. But I meant every word.",
	"Sorry, said slowly, / like rain that knows it was wrong. / Can the sun come back?",
	"Whereas I was wrong, and whereas I miss you, the petitioner humbly asks the Court of {name} to reconsider.",
	"Today’s forecast: heavy remorse, clearing by evening if you smile.",
	"Forgiveness, serves two: / one honest apology / a pinch of patience / a lot of listening",
	"It’s okay to need time. I’ll be right here. (The other button is, too.)",
}

// DefaultFallbackYes is the yes card's note when Claude can't write.
const DefaultFallbackYes = `Thank you. Truly. That means more than I can put on a page.
— I’ll listen before I explain.
— I’ll keep the little promises, not just the big ones.
— I’ll make it up to you, starting with your favourite dinner.`

// BargainRounds is how many times the offers can be sweetened, the first
// round included.
const BargainRounds = 3

// Themes are the palettes the page knows.
var Themes = []string{"rose", "sage", "ocean", "dusk", "gold"}

const (
	maxNameLen     = 64
	maxTitleLen    = 120
	maxMessageLen  = 4000
	maxLabelLen    = 40
	maxLabels      = 20
	maxQuestionLen = 200
	maxLineLen     = 400
	maxStyles      = 30
	maxFallbacks   = 50
	maxContextLen  = 2000
	maxPromptLen   = 8000
	maxGiftLen     = 500
	maxSealLen     = 8
	maxStampLen    = 24
	maxReasons     = 8
	maxImages      = 200
	maxOfferLen    = 300
	minPinLen      = 4
	maxPinLen      = 64
)

// Config is the validated application configuration.
type Config struct {
	Host string
	Port string

	// Name is who the letter is for, From who signs it. Title, Message (blank
	// lines between paragraphs) and Question make the page; Yes and NoLabels
	// are its two answers (NoLabels[i] after i noes; once they run out only
	// yes is left).
	Name     string
	From     string
	Title    string
	Message  string
	Question string
	Yes      string
	NoLabels []string

	// YesTitle and Stamp dress the card shown after a yes. Gift is an
	// optional extra on it (a code or a link), sent only with the yes;
	// GiftIntro announces it.
	YesTitle  string
	Stamp     string
	Gift      string
	GiftIntro string

	// Seal is the character(s) on the envelope's wax seal, Theme the palette.
	Seal  string
	Theme string

	Location *time.Location

	// Pin, when set, locks the page: only a sealed envelope and a code field
	// are served until someone enters it.
	Pin string

	// AI has Claude, through Claude Code on the container's login, write a
	// note after every answer. Context tells it what happened (never shown);
	// Prompt adds instructions. NoStyles are the forms of the notes after
	// each no, YesStyle the one after a yes.
	AI       bool
	Context  string
	Prompt   string
	NoStyles []string
	// NoLastStyle is the form after the last no; Shuffle draws NoStyles in a
	// new order on each visit (in order otherwise). Images are small images
	// Claude weaves in, one per note, drawn at random.
	NoLastStyle string
	Shuffle     bool
	Images      []string
	YesStyle    string
	// Why is the prompt over the Reasons chips offered after a no (AskWhy).
	AskWhy  bool
	Why     string
	Reasons []string
	// Bargain offers to make it up to them: Offer opens it, More sweetens it,
	// Offers are ideas the author is happy to do (for Claude only).
	Bargain bool
	Offer   string
	More    string
	Offers  []string
	// Cloud lets the little cloud over the sun add an aside to the notes.
	Cloud   bool
	Model   string
	Command string
	// FallbackNo and FallbackYes stand in when Claude can't write.
	FallbackNo     []string
	FallbackYes    string
	FallbackWhy    []string
	FallbackOffers []string
	FallbackDeal   string
	FallbackCloud  []string

	// LockPath is a lock file next to the Claude login: Claude Code runs take
	// turns on it across replicas sharing the volume, since two renewals of
	// the login at once can revoke one another.
	LockPath string
	// AnswersPath is a Markdown file next to the login where every answer
	// is noted for the page's owner. Empty when LETTER_ANSWERS is false.
	AnswersPath string
	// NotesPath keeps the last notes written, so a new visit never repeats
	// an old one. Empty without a Claude config directory.
	NotesPath string
}

// Load builds the configuration from getenv (os.Getenv in production) and
// reports every invalid setting at once.
func Load(getenv func(string) string) (Config, error) {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	cfg := Config{
		Host:      getenv("LETTER_HOST"),
		Port:      valueOr(get("LETTER_PORT"), "8080"),
		Name:      valueOr(get("LETTER_NAME"), DefaultName),
		From:      get("LETTER_FROM"),
		Title:     valueOr(get("LETTER_TITLE"), DefaultTitle),
		Message:   valueOr(normalize(getenv("LETTER_MESSAGE")), DefaultMessage),
		Question:  valueOr(get("LETTER_QUESTION"), DefaultQuestion),
		Yes:       valueOr(get("LETTER_YES"), DefaultYes),
		YesTitle:  valueOr(get("LETTER_YES_TITLE"), DefaultYesTitle),
		Stamp:     valueOr(get("LETTER_STAMP"), DefaultStamp),
		Gift:      get("LETTER_GIFT"),
		GiftIntro: valueOr(get("LETTER_GIFT_INTRO"), DefaultGiftIntro),
		Seal:      valueOr(get("LETTER_SEAL"), DefaultSeal),
		Theme:     strings.ToLower(valueOr(get("LETTER_THEME"), DefaultTheme)),
		Pin:       get("LETTER_PIN"),
		Context:   get("LETTER_CONTEXT"),
		Prompt:    get("LETTER_PROMPT"),
		YesStyle:  valueOr(get("LETTER_YES_STYLE"), DefaultYesStyle),
		Why:       valueOr(get("LETTER_WHY"), "Why not?"),
		Offer:     valueOr(get("LETTER_OFFER"), "Make me an offer"),
		More:      valueOr(get("LETTER_MORE"), "Sweeten it"),

		NoLastStyle:    valueOr(get("LETTER_NO_LAST_STYLE"), DefaultNoLastStyle),
		Images:         linesOr(getenv("LETTER_IMAGES"), DefaultImages),
		Reasons:        linesOr(getenv("LETTER_REASONS"), DefaultReasons),
		Offers:         linesOr(getenv("LETTER_OFFERS"), nil),
		FallbackWhy:    linesOr(getenv("LETTER_FALLBACK_WHY"), DefaultFallbackWhy),
		FallbackOffers: linesOr(getenv("LETTER_FALLBACK_OFFERS"), DefaultFallbackOffers),
		FallbackDeal:   valueOr(get("LETTER_FALLBACK_DEAL"), DefaultFallbackDeal),
		FallbackCloud:  linesOr(getenv("LETTER_FALLBACK_CLOUD"), DefaultFallbackCloud),
		Model:          valueOr(get("LETTER_MODEL"), DefaultModel),
		Command:        valueOr(get("LETTER_COMMAND"), DefaultCommand),

		NoLabels:    linesOr(getenv("LETTER_NO"), DefaultNoLabels),
		NoStyles:    linesOr(getenv("LETTER_NO_STYLES"), DefaultNoStyles),
		FallbackNo:  linesOr(getenv("LETTER_FALLBACK_NO"), DefaultFallbackNo),
		FallbackYes: valueOr(normalize(getenv("LETTER_FALLBACK_YES")), DefaultFallbackYes),
	}

	var errs []error
	tooLong := func(key, v string, n int) {
		if utf8.RuneCountInString(v) > n {
			errs = append(errs, fmt.Errorf("%s: at most %d characters", key, n))
		}
	}
	tooMany := func(key string, lines []string, count, n int) {
		long := func(l string) bool { return utf8.RuneCountInString(l) > n }
		if len(lines) > count || slices.ContainsFunc(lines, long) {
			errs = append(errs, fmt.Errorf("%s: at most %d lines of %d characters", key, count, n))
		}
	}

	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("LETTER_PORT %q: want a port number", cfg.Port))
	}
	tooLong("LETTER_NAME", cfg.Name, maxNameLen)
	tooLong("LETTER_FROM", cfg.From, maxNameLen)
	tooLong("LETTER_TITLE", cfg.Title, maxTitleLen)
	tooLong("LETTER_MESSAGE", cfg.Message, maxMessageLen)
	tooLong("LETTER_QUESTION", cfg.Question, maxQuestionLen)
	tooLong("LETTER_YES", cfg.Yes, maxLabelLen)
	tooMany("LETTER_NO", cfg.NoLabels, maxLabels, maxLabelLen)
	tooLong("LETTER_YES_TITLE", cfg.YesTitle, maxTitleLen)
	tooLong("LETTER_STAMP", cfg.Stamp, maxStampLen)
	tooLong("LETTER_GIFT", cfg.Gift, maxGiftLen)
	tooLong("LETTER_GIFT_INTRO", cfg.GiftIntro, maxTitleLen)
	tooLong("LETTER_SEAL", cfg.Seal, maxSealLen)
	tooLong("LETTER_CONTEXT", cfg.Context, maxContextLen)
	tooLong("LETTER_PROMPT", cfg.Prompt, maxPromptLen)
	tooLong("LETTER_YES_STYLE", cfg.YesStyle, maxLineLen)
	tooMany("LETTER_NO_STYLES", cfg.NoStyles, maxStyles, maxLineLen)
	tooMany("LETTER_FALLBACK_NO", cfg.FallbackNo, maxFallbacks, maxLineLen)
	tooLong("LETTER_FALLBACK_YES", cfg.FallbackYes, maxMessageLen)
	tooLong("LETTER_NO_LAST_STYLE", cfg.NoLastStyle, maxLineLen)
	tooMany("LETTER_IMAGES", cfg.Images, maxImages, maxLabelLen*2)
	tooLong("LETTER_WHY", cfg.Why, maxLabelLen)
	tooMany("LETTER_REASONS", cfg.Reasons, maxReasons, maxLabelLen)
	tooLong("LETTER_OFFER", cfg.Offer, maxLabelLen)
	tooLong("LETTER_MORE", cfg.More, maxLabelLen)
	tooMany("LETTER_OFFERS", cfg.Offers, maxFallbacks, maxOfferLen)
	tooMany("LETTER_FALLBACK_WHY", cfg.FallbackWhy, maxFallbacks, maxLineLen)
	tooMany("LETTER_FALLBACK_OFFERS", cfg.FallbackOffers, maxFallbacks, maxOfferLen)
	tooLong("LETTER_FALLBACK_DEAL", cfg.FallbackDeal, maxLineLen)
	tooMany("LETTER_FALLBACK_CLOUD", cfg.FallbackCloud, maxFallbacks, maxLabelLen*3)
	if len(cfg.FallbackOffers) < 3 {
		errs = append(errs, errors.New("LETTER_FALLBACK_OFFERS: at least 3 lines"))
	}

	if n := utf8.RuneCountInString(cfg.Pin); n > 0 && (n < minPinLen || n > maxPinLen) {
		errs = append(errs, fmt.Errorf("LETTER_PIN: %d to %d characters", minPinLen, maxPinLen))
	}
	if !slices.Contains(Themes, cfg.Theme) {
		errs = append(errs, fmt.Errorf("LETTER_THEME %q: want one of %s", cfg.Theme, strings.Join(Themes, ", ")))
	}
	if ai, err := boolOr(get("LETTER_AI"), true); err != nil {
		errs = append(errs, fmt.Errorf("LETTER_AI %q: want true or false", get("LETTER_AI")))
	} else {
		cfg.AI = ai
	}
	for key, dst := range map[string]*bool{
		"LETTER_SHUFFLE": &cfg.Shuffle, "LETTER_ASK_WHY": &cfg.AskWhy, "LETTER_BARGAIN": &cfg.Bargain, "LETTER_CLOUD": &cfg.Cloud,
	} {
		if v, err := boolOr(get(key), true); err != nil {
			errs = append(errs, fmt.Errorf("%s %q: want true or false", key, get(key)))
		} else {
			*dst = v
		}
	}
	if strings.HasPrefix(cfg.Model, "-") || strings.ContainsAny(cfg.Model, " \t\n") {
		errs = append(errs, fmt.Errorf("LETTER_MODEL %q: want a model name such as sonnet", cfg.Model))
	}
	dir := claudeDir(getenv)
	if dir != "" {
		cfg.LockPath = filepath.Join(dir, "letter-claude.lock")
		cfg.NotesPath = filepath.Join(dir, "letter-notes.json")
	}
	if keep, err := boolOr(get("LETTER_ANSWERS"), true); err != nil {
		errs = append(errs, fmt.Errorf("LETTER_ANSWERS %q: want true or false", get("LETTER_ANSWERS")))
	} else if keep && dir != "" {
		cfg.AnswersPath = filepath.Join(dir, "letter-answers.md")
	}

	tz := valueOr(get("LETTER_TIMEZONE"), "UTC")
	if loc, err := time.LoadLocation(tz); err != nil {
		errs = append(errs, fmt.Errorf("LETTER_TIMEZONE %q: want an IANA name such as Europe/Paris", tz))
	} else {
		cfg.Location = loc
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Paragraphs splits the letter on blank lines.
func (c Config) Paragraphs() []string {
	var out []string
	for p := range strings.SplitSeq(c.Message, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NoLabel is the no button's word after noes answers.
func (c Config) NoLabel(noes int) string {
	return c.NoLabels[min(max(noes, 0), len(c.NoLabels)-1)]
}

// normalize trims a multi-line value and its lines' trailing spaces, with
// Unix line endings, so paragraphs split the same from any editor.
func normalize(v string) string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	lines := strings.Split(v, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// linesOr splits v into its non-blank lines, or returns fallback for none.
func linesOr(v string, fallback []string) []string {
	var out []string
	for line := range strings.Lines(v) {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return slices.Clone(fallback)
	}
	return out
}

// claudeDir is Claude Code's config directory, where its login lives.
func claudeDir(getenv func(string) string) string {
	if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".claude")
	}
	return ""
}

func boolOr(v string, fallback bool) (bool, error) {
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseBool(v)
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
