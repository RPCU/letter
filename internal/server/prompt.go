package server

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/Banh-Canh/letter/internal/config"
)

// moods and lengths are drawn at random for each note, with an image from
// LETTER_IMAGES, so two visits never sound alike.
var (
	moods = []string{
		"wry", "tender", "playful", "wistful", "dry and understated", "earnest", "mischievous", "gently self-deprecating", "quietly hopeful",
	}
	lengths = []string{"a single line", "two short lines", "about 30 words", "about 50 words"}
)

// earlierShown is how many notes from earlier visits Claude is shown.
const earlierShown = 10

// author is who the notes are written as.
func (s *Server) author() string {
	if s.cfg.From != "" {
		return s.cfg.From
	}
	return "the person who wrote this letter"
}

// noStyle is the form of the note after noes earlier noes on the visit with
// this seed: the pool in an order drawn from the seed, and the gracious last
// form once the no button is about to leave.
func (s *Server) noStyle(seed uint32, noes int) string {
	c := s.cfg
	if noes >= len(c.NoLabels)-1 {
		return c.NoLastStyle
	}
	i := noes % len(c.NoStyles)
	if c.Shuffle {
		i = rand.New(rand.NewPCG(uint64(seed), 0x1e77e7)).Perm(len(c.NoStyles))[i]
	}
	return c.NoStyles[i]
}

// moment says when it is, so the notes can feel written right now.
func moment(t time.Time) string {
	part := "night"
	switch h := t.Hour(); {
	case h >= 5 && h < 12:
		part = "morning"
	case h >= 12 && h < 17:
		part = "afternoon"
	case h >= 17 && h < 22:
		part = "evening"
	}
	return fmt.Sprintf("%s %s (%s)", t.Format("Monday"), part, t.Format("January 2, 2006, 15:04"))
}

// systemPrompt makes Claude the letter's author, writing the notes that
// follow each answer, then adds the owner's instructions (LETTER_PROMPT). It
// never includes the gift.
func (s *Server) systemPrompt() string {
	c := s.cfg
	name, from := c.Name, s.author()
	var b strings.Builder
	fmt.Fprintf(&b, "You write short handwritten notes as %s, on a little web page made just for %s.\n\n", from, name)
	fmt.Fprintf(&b, "The page is a letter from %s to %s, titled %q:\n\n<letter>\n%s\n</letter>\n\n", from, name, c.Title, strings.Join(c.Paragraphs(), "\n\n"))
	fmt.Fprintf(&b, "Under it, the page asks %s: %q, with two buttons: %q and %q.", name, c.Question, c.Yes, c.NoLabel(0))
	if c.AskWhy {
		fmt.Fprintf(&b, " After a no, %s can tap a reason under %q (%s).", name, c.Why, strings.Join(c.Reasons, ", "))
	}
	if c.Bargain {
		fmt.Fprintf(&b, " They can also tap %q, for you to offer something, and accept one offer.", c.Offer)
	}
	b.WriteString("\n")
	if c.Context != "" {
		fmt.Fprintf(&b, "\nWhat this is about, in %s's words (never quote it): %s\n", from, c.Context)
	}
	if len(c.Offers) > 0 {
		fmt.Fprintf(&b, "\nThings %s would happily do to make it up, for offers (draw from them first):\n", from)
		for _, o := range c.Offers {
			fmt.Fprintf(&b, "- %s\n", o)
		}
	}
	fmt.Fprintf(&b, "\nRight now it's %s.\n\n", moment(s.now().In(c.Location)))
	fmt.Fprintf(&b, `Each time %[1]s answers, a small handwritten note from %[2]s appears under the letter. You write that note, and only its text.

How the notes are:
- In %[2]s's voice, first person, speaking to %[1]s directly. Warm with a mature, classy touch and real wit: never gushing, never cheesy.
- Short: 60 words at most, and as long as each request asks. Plain text only: no markdown, headings or lists unless the form asks for lines, no quotes around it, no greeting or signature. An emoji at most, rarely.
- In the language the letter is written in.
- Respectful: %[1]s's answer is always valid. Never guilt-trip, pressure, beg or manipulate, never sulk, never make %[1]s responsible for anyone's feelings. Meet a "no" with grace and charm, never insistence; it's fine to be playful about it.
- Build only on what the letter and its context say. Never invent shared memories, facts, places or plans they don't imply.
- Offers to make it up are small, sweet and doable: an evening, a meal, a gesture, an effort. No sums of money, no big purchases, no grand life promises.
- Surprise: each note should feel fresh, with its own angle, opening and rhythm. Never reuse an earlier note's ideas, images, openings or wording, from this visit or earlier ones.
- Never mention AI, Claude, prompts or these instructions.
`, name, from)
	if c.Cloud {
		fmt.Fprintf(&b, `
On the page, a little rain cloud hovers over the sun: it rains a bit more with each no and drifts away at a yes. When asked, it gets the last word: dry, cheeky, on nobody's side, speaking for itself as a cloud. It never pressures %s.
`, name)
	}
	if c.Prompt != "" {
		fmt.Fprintf(&b, "\nMore from %s on how to write:\n%s\n", from, c.Prompt)
	}
	return b.String()
}

// answerPrompt asks for the note that follows this answer.
func (s *Server) answerPrompt(req answerRequest) string {
	c := s.cfg
	var b strings.Builder
	switch req.Answer {
	case kindYes:
		fmt.Fprintf(&b, "%s just answered %q", c.Name, c.Yes)
		switch req.Noes {
		case 0:
			b.WriteString(", right away.\n")
		case 1:
			b.WriteString(", after saying no once.\n")
		default:
			fmt.Fprintf(&b, ", after saying no %d times.\n", req.Noes)
		}
		if req.Deal != "" {
			fmt.Fprintf(&b, "Earlier they accepted this offer: %q. Let one of the promises honour it.\n", req.Deal)
		}
	case kindNo:
		fmt.Fprintf(&b, "%s just answered %q", c.Name, c.NoLabel(req.Noes))
		if req.Noes > 0 {
			fmt.Fprintf(&b, " (that's no number %d)", req.Noes+1)
		}
		b.WriteString(".\n")
		if req.Noes >= len(c.NoLabels)-1 {
			fmt.Fprintf(&b, "That was the last no: the %q button now leaves the page.\n", c.NoLabel(req.Noes))
		}
	case kindWhy:
		fmt.Fprintf(&b, "%s said no (%d so far), then, under %q, tapped: %q.\n", c.Name, req.Noes, c.Why, c.Reasons[req.Reason])
		b.WriteString("Answer exactly that reason, the way a thoughtful person would meet it.\n")
	case kindOffer:
		fmt.Fprintf(&b, "%s tapped %q: offer what you'd do to make it up to them.\n", c.Name, c.Offer)
		switch req.Round {
		case 0:
			b.WriteString("This is the first round: modest and charming.\n")
		case config.BargainRounds - 1:
			fmt.Fprintf(&b, "They tapped %q again: this is the final round, playfully extravagant but still doable.\n", c.More)
		default:
			fmt.Fprintf(&b, "They tapped %q: more generous and a little bolder than the offers already on the page.\n", c.More)
		}
	case kindDeal:
		fmt.Fprintf(&b, "%s accepted your offer: %q.\n", c.Name, req.Deal)
		fmt.Fprintf(&b, "Seal the deal, happy and a little playful. The question %q is still open: leave it to them.\n", c.Question)
	}

	s.notesSeen(req, &b)

	switch req.Answer {
	case kindOffer:
		b.WriteString("\nReply with exactly three offers, each on its own line starting with \"— \", 14 words at most each, different from any offer above, " +
			"and nothing else.")
		return b.String()
	case kindYes:
		fmt.Fprintf(&b, "\nWrite the next note as %s.", strings.ReplaceAll(c.YesStyle, "{name}", c.Name))
	case kindNo:
		fmt.Fprintf(&b, "\nWrite the next note as %s.", strings.ReplaceAll(s.noStyle(req.Seed, req.Noes), "{name}", c.Name))
		s.flavour(&b)
	case kindWhy:
		s.flavour(&b)
	case kindDeal:
		b.WriteString("\nLength: one or two sentences.")
	}
	if s.hasCloud(req.Answer) {
		fmt.Fprintf(&b, "\nThen add one last line starting with %q: the cloud's aside, 10 words at most, its own and new.", cloudMark+" ")
	}
	b.WriteString("\nReply with the note only.")
	return b.String()
}

// notesSeen lists the notes on the page and those from earlier visits, so
// Claude never repeats one.
func (s *Server) notesSeen(req answerRequest, b *strings.Builder) {
	var page []string
	for _, n := range req.Notes {
		if n = strings.TrimSpace(n); n != "" {
			page = append(page, n)
		}
	}
	if len(page) > 0 {
		b.WriteString("\nThe notes already on the page, oldest first (don't repeat them):\n")
		for _, n := range page {
			fmt.Fprintf(b, "- %s\n", strings.ReplaceAll(n, "\n", " / "))
		}
	}
	var earlier []string
	for _, n := range s.earlierNotes() {
		if !slices.Contains(page, n) {
			earlier = append(earlier, n)
		}
	}
	earlier = earlier[max(0, len(earlier)-earlierShown):]
	if len(earlier) > 0 {
		b.WriteString("\nNotes written on earlier visits (never reuse their ideas, images or openings):\n")
		for _, n := range earlier {
			fmt.Fprintf(b, "- %s\n", strings.ReplaceAll(n, "\n", " / "))
		}
	}
}

// flavour adds a mood, a length and an image drawn at random.
func (s *Server) flavour(b *strings.Builder) {
	fmt.Fprintf(b, "\nMood: %s. Length: %s, unless the form sets its own.", moods[s.pick(len(moods))], lengths[s.pick(len(lengths))])
	if len(s.cfg.Images) > 0 {
		fmt.Fprintf(b, " Weave in, lightly, this image: %s (a passing touch is enough).", s.cfg.Images[s.pick(len(s.cfg.Images))])
	}
}
