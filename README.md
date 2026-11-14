# ✉️ Letter

A little web page holding a sealed letter and one question, for one person.
They open the envelope, read what you wrote, and answer. After every answer,
Claude writes them a short handwritten note in your voice.

Every word on the page is yours to set, so the same page can carry:

- an apology: *"I'm sorry … Do you forgive me?"* (the default),
- a send-off: *"Bon voyage … Will you send me a photo from the beach?"*,
- an invitation: *"Dinner on Friday?"*,
- a thank-you, a question you've been meaning to ask, or anything else that
  fits in a letter and a yes or no.

It's one static Go binary (standard library only, with the template, assets
and time zone database embedded), configured entirely through environment
variables. The notes are written by Claude through Claude Code on a Claude
login (no API key). Without Claude, canned notes stand in, so the page always
works.

## How it works

1. **The envelope.** A wax seal (`LETTER_SEAL`) to tap. With `LETTER_PIN`
   set, a code is asked first (see [Locking it](#locking-it)).
2. **The letter.** It unfolds: the date, your title, *Dear {name},* your
   paragraphs and your signature.
3. **The question**, with a little cloud over the sun, and two buttons.
4. **No** → Claude writes a short note in a new form each time, drawn in a
   new order on every visit: a haiku, a telegram (STOP), a lost-and-found
   notice, a museum placard, terms and conditions, a horoscope, a menu… The
   last No always gets a gracious "take your time". Each note also gets a
   random mood, length and small image to weave in, knows the time of day,
   and never repeats a note from this visit or earlier ones. Each No adds
   rain to the cloud, shrinks No a little and grows Yes. Once the No labels
   run out, the No button slips away and only Yes is left.
   - **Why not?** Under each No note, a few reasons to tap. Claude answers
     the one tapped.
   - **Make me an offer.** After the first No, Claude offers three small,
     doable ways to make it up (or to earn the yes). *Sweeten it* asks for
     bolder ones, for three rounds. Tapping one seals the deal: Claude writes
     a note, Yes grows, and the note after a yes honours it.
   - **The cloud talks.** The cloud over the sun gets the last word after
     each note, in a speech bubble: dry, cheeky, on nobody's side.
5. **Yes** → the cloud drifts off, petals and hearts fly, a stamp
   (`LETTER_STAMP`) lands, and Claude writes a thank-you with three small
   promises (or wishes, or whatever `LETTER_YES_STYLE` asks for). Tap each
   one to accept it. The last one reveals the optional gift (`LETTER_GIFT`):
   blurred until tapped, then tap to copy.

There is no prompt box: Claude only reacts to what's tapped. It's told to
stay warm, a little witty and respectful. A No is always a valid answer:
no guilt-tripping, no pressure, no invented memories.

## Configuration

Every setting is optional. Multi-line values: in `LETTER_MESSAGE` blank lines
separate paragraphs, and the list settings take one item per line. `{name}`
in the styles and canned notes becomes `LETTER_NAME`.

**The letter**

| Variable              | Default                        | Description |
| --------------------- | ------------------------------ | ----------- |
| `LETTER_NAME`         | `you`                          | Who the letter is for (max 64). |
| `LETTER_FROM`         |                                | Your signature; Claude writes as you (max 64). |
| `LETTER_TITLE`        | `I’m sorry`                    | The letter's heading (max 120). |
| `LETTER_MESSAGE`      | a short apology                | The letter, paragraphs separated by blank lines (max 4000). |
| `LETTER_QUESTION`     | `Do you forgive me?`           | The question under it (max 200). |
| `LETTER_YES`          | `Yes`                          | The yes button (max 40). |
| `LETTER_NO`           | `No` … `Last chance…` (6)      | The no button's words, one per line, one more after each No. Once they run out, the button leaves (max 20 × 40). |
| `LETTER_YES_TITLE`    | `Thank you`                    | Heading of the card after the yes. |
| `LETTER_STAMP`        | `forgiven`                     | The stamp on that card (max 24). |
| `LETTER_GIFT`         |                                | An extra revealed after the yes and the promises (a code or a link). Never in the page: only in the reply to the yes. Never shown to Claude (max 500). |
| `LETTER_GIFT_INTRO`   | `And a little something, to make it up to you` | The line announcing it. |
| `LETTER_SEAL`         | `🥺`                           | On the wax seal: a character or an emoji (max 8). |
| `LETTER_THEME`        | `rose`                         | `rose`, `sage`, `ocean`, `dusk` or `gold`. |
| `LETTER_TIMEZONE`     | `UTC`                          | IANA zone for the letter's date, the time of day Claude knows, and the answers log. |
| `LETTER_PIN`          |                                | A code that locks the page (4 to 64 characters; digits bring up the number pad). |

**Claude's notes**

| Variable              | Default                        | Description |
| --------------------- | ------------------------------ | ----------- |
| `LETTER_AI`           | `true`                         | Claude writes the notes; `false` uses only the canned ones. |
| `LETTER_CONTEXT`      |                                | What this is about, for Claude only, never shown: *"I forgot our dinner on Friday"* (max 2000). |
| `LETTER_PROMPT`       |                                | More instructions on how to write: in-jokes, nicknames, things to avoid (max 8000). |
| `LETTER_NO_STYLES`    | twenty forms                   | The forms a No note can take, one per line. |
| `LETTER_NO_LAST_STYLE`| a gracious "take your time"    | The form after the last No, as the No button leaves. |
| `LETTER_SHUFFLE`      | `true`                         | Draw the forms in a new order each visit; `false` uses them in order. |
| `LETTER_IMAGES`       | 36 small images                | One is drawn for each note for Claude to weave in (a paper boat, a lighthouse…). |
| `LETTER_YES_STYLE`    | thank-you + three promises     | The form of the note after the yes. Lines starting with `— ` become the items to accept. |
| `LETTER_MODEL`        | `sonnet`                       | Claude model (`sonnet`, `opus`, `haiku` or a full model ID). |
| `LETTER_COMMAND`      | `claude`                       | The Claude Code CLI. |

**Interactions**

| Variable              | Default                        | Description |
| --------------------- | ------------------------------ | ----------- |
| `LETTER_ASK_WHY`      | `true`                         | Offer reasons to tap after a No. |
| `LETTER_WHY`          | `Why not?`                     | The line over them. |
| `LETTER_REASONS`      | `Too soon` … `Convince me` (4) | The reasons, one per line (max 8 × 40). |
| `LETTER_BARGAIN`      | `true`                         | Offer the bargain after the first No. |
| `LETTER_OFFER`        | `Make me an offer`             | The link that opens it. |
| `LETTER_MORE`         | `Sweeten it`                   | The link for bolder offers. |
| `LETTER_OFFERS`       |                                | Things you'd happily do, for Claude to draw the offers from (for Claude only). Otherwise it invents small, doable ones from the context. |
| `LETTER_CLOUD`        | `true`                         | Let the cloud add its aside after the notes. |

**Canned notes** (when Claude can't write)

| Variable              | Default                        | Description |
| --------------------- | ------------------------------ | ----------- |
| `LETTER_FALLBACK_NO`  | six notes                      | One per No in turn; the last one stays. |
| `LETTER_FALLBACK_YES` | thank-you + three promises     | The note after the yes. |
| `LETTER_FALLBACK_WHY` | one per default reason         | Answers to the reasons, in the same order; the last one stays. |
| `LETTER_FALLBACK_OFFERS` | nine offers                 | Three per round (at least three lines). |
| `LETTER_FALLBACK_DEAL`| a "deal" note                  | Seals a deal. |
| `LETTER_FALLBACK_CLOUD` | six asides                   | The cloud's asides, drawn at random. |

**Server**

| Variable              | Default                        | Description |
| --------------------- | ------------------------------ | ----------- |
| `LETTER_ANSWERS`      | `true`                         | Log every answer for you (see [Their answers](#their-answers)). |
| `LETTER_HOST`         | all interfaces                 | Listen address. |
| `LETTER_PORT`         | `8080`                         | Listen port. |

An invalid value stops the server at startup and lists every problem.

## Examples

### An apology, made personal

The defaults already make an apology; give Claude what it needs to make it
yours:

```yaml
LETTER_NAME: Sam
LETTER_FROM: Alex
LETTER_CONTEXT: I missed Sam's birthday dinner because I stayed late at work, and didn't call.
LETTER_PROMPT: Sam loves bad puns and hates being called "babe".
LETTER_OFFERS: |
  cook Sam's favourite risotto
  a weekend with no work laptop
LETTER_GIFT: https://example.com/concert-tickets
LETTER_GIFT_INTRO: And two tickets, for the one I owe you
```

### An invitation

```yaml
LETTER_NAME: Sam
LETTER_FROM: Alex
LETTER_TITLE: A small question
LETTER_MESSAGE: |
  There's a little place by the river that does the best dumplings in town.

  I'd love to take you there.
LETTER_QUESTION: Dinner on Friday?
LETTER_YES: I'd love to
LETTER_NO: |
  Can't
  Busy
  Maybe another time
LETTER_YES_TITLE: Friday, then
LETTER_STAMP: it's a date
LETTER_SEAL: 🥟
LETTER_THEME: gold
LETTER_REASONS: |
  Busy that night
  Not a dumpling person
  Convince me
LETTER_FALLBACK_WHY: |
  Then name the night, I'll move the dumplings.
  There are noodles too. I checked.
  Exhibit A: dumplings. Exhibit B: me, on time.
LETTER_OFFER: What would make it a yes?
LETTER_YES_STYLE: >-
  a happy one-line reply, then exactly three little plans for the evening,
  each on its own line starting with "— "
LETTER_FALLBACK_YES: |
  Friday it is. I'll book the table by the window.
  — I pick you up at seven.
  — We order too many dumplings.
  — We walk back along the river.
```

### A good trip

```yaml
LETTER_NAME: Sam
LETTER_FROM: Alex
LETTER_TITLE: Bon voyage
LETTER_MESSAGE: |
  Two weeks of sea, sun and long lunches.

  Have the most wonderful trip. Eat everything, sleep in a little, and come
  back with stories.
LETTER_QUESTION: Will you send me a photo from the beach?
LETTER_YES: Promise
LETTER_NO: |
  Nope
  Too busy
  Maybe later
LETTER_YES_TITLE: Safe travels
LETTER_STAMP: bon voyage
LETTER_SEAL: ✈️
LETTER_THEME: ocean
LETTER_CONTEXT: Sam is off on two weeks of holiday by the sea.
LETTER_NO_STYLES: |
  a playful one-line bribe involving ice cream
  a haiku about a postcard that never arrives
  a fake customs declaration for the photo
  a weather report from the beach
LETTER_NO_LAST_STYLE: a gracious note: no photo needed, just come back rested and happy
LETTER_IMAGES: |
  a beach umbrella
  a seashell
  a ferry
  a melting ice cream
  a sunset
LETTER_REASONS: |
  No wifi
  Too busy relaxing
  Convince me
LETTER_FALLBACK_WHY: |
  The café on the corner has wifi. I checked. Twice.
  Fair. Relax first, photos later.
  One photo, one ice cream. My treat when you're back.
LETTER_OFFER: What do I get for it?
LETTER_OFFERS: |
  an airport pick-up on the way back
  a favourite dinner, my treat
LETTER_YES_STYLE: >-
  a warm, happy send-off of one or two sentences, then exactly three little
  wishes for the trip, each on its own line starting with "— "
LETTER_FALLBACK_NO: |
  I'll trade you an ice cream for one photo. Final offer.
  A postcard drifts by, / lost somewhere over the waves. / I'll still wait for it.
  No photo needed. Just come back happy.
LETTER_FALLBACK_YES: |
  Deal! Have the best time.
  — Warm evenings and cold drinks.
  — Every ferry on time.
  — A suitcase that closes on the way back.
```

## Locking it

Set `LETTER_PIN` and the page serves only a sealed envelope with a code field
("It’s sealed with a little code"): no name, no letter, no question, and the
link preview says only "A sealed letter". The right code cracks the seal and
the letter opens; a wrong one shakes the envelope. Answers are refused
without it too.

Once entered, an `HttpOnly` cookie keeps it open for 90 days on that browser.
The cookie is derived from the code (not the code itself), so it works on
every replica, and changing `LETTER_PIN` locks everyone out again. Guesses
are throttled across every visitor (8, then 4 a minute), so a 4-digit code
takes days to guess and a 6-digit one years; a word is better still. Send the
code separately from the link, and keep it in a secret rather than in plain
configuration.

## Claude

Each answer runs a fresh `claude --print` in an empty directory, cut off from
the account it runs on: no tools, no MCP servers or claude.ai connectors, no
plugins, skills, hooks, settings, `CLAUDE.md` or memory, no saved session,
its own system prompt instead of Claude Code's, and none of the `LETTER_*`
variables. It knows the letter, the question, the answer, how many times they
said no, the notes already on the page and the last ones from earlier visits
(so it never repeats itself), the time of day, `LETTER_CONTEXT`,
`LETTER_OFFERS` and `LETTER_PROMPT`, and never the gift or the code.

Notes stream in. Answers are throttled across every visitor (20 in a burst,
then 8 a minute, at most 2 notes written at once). Claude Code runs take
turns on a lock file next to the login (`$CLAUDE_CONFIG_DIR/letter-claude.lock`),
so replicas sharing it never renew the login at once (two renewals at once
can sign it out). When Claude fails (not signed in, missing, out of usage),
the canned notes stand in after a short pause, a warning is logged, and
Claude is tried again a minute later.

**Signing in.** The image ships Claude Code and a shell. Its login lives in
`CLAUDE_CONFIG_DIR` (`/home/letter/.claude`). Sign in once from inside the
container:

```sh
docker exec -it <container> claude auth login     # or: kubectl exec -it <pod> -- claude auth login
```

Mount a volume on `/home/letter/.claude` to keep the login, the answers log
and the recent notes across restarts; several replicas need one they can all
write (ReadWriteMany). Or set `CLAUDE_CODE_OAUTH_TOKEN` (from
`claude setup-token`) from a secret. Each note runs a Claude Code process of
about 250 MiB, two at most at once: give the container ~640 MiB and 2 CPUs.

### Their answers

Every answer is added to `$CLAUDE_CONFIG_DIR/letter-answers.md`, next to the
login, with the time:

```
- 2026-11-14 21:30 CET: no ("No", no number 1)
- 2026-11-14 21:30 CET: why not: "Still a bit upset"
- 2026-11-14 21:31 CET: asked for an offer (round 1)
- 2026-11-14 21:31 CET: **deal**: "Brunch this weekend, my cooking, phone face-down."
- 2026-11-14 21:32 CET: **yes** ("Yes", after 1 no), with the deal "Brunch this weekend, my cooking, phone face-down."
```

It's never cleared and never served. Read it from the container:

```sh
docker exec <container> cat /home/letter/.claude/letter-answers.md
```

## Running it

```sh
docker run -p 8080:8080 \
  -v letter-claude:/home/letter/.claude \
  -e LETTER_NAME=Sam -e LETTER_FROM=Alex -e LETTER_TIMEZONE=Europe/Paris \
  <image>
```

The image's default command starts the server. It listens on `LETTER_PORT`
(8080) and answers `GET /healthz` for liveness and readiness probes. Put it
behind HTTPS: the code's cookie is `Secure` when the request came over TLS
(directly or through `X-Forwarded-Proto: https`), and notes stream for up to
two minutes, so give the proxy a request timeout of at least that.

## Endpoints

- `GET /`: the page (only the sealed envelope while locked).
- `POST /api/answer` (`{"answer": "yes"|"no"|"why"|"offer"|"deal", "noes": n,
  "seed": n, "notes": ["…"], "reason": i, "round": i, "deal": "…"}`): the
  note that follows, streamed as JSON lines (`{"text"}`…, then the cloud's
  `{"cloud"}`, then `{"done": true}`, with `gift`, `link` and `intro` after a
  yes when there's a gift; or `{"error": "away"}` if Claude stops mid-note).
  `seed` is drawn once per visit and orders the No forms; `why` takes the
  reason's index (after a No), `offer` the round (0 to 2), `deal` the accepted
  offer (also sent with the yes). Same-origin only. `409` for a No once the No
  labels have run out (or a disabled feature), `429` when throttled, `403`
  while locked.
- `POST /api/unlock` (`{"pin": "…"}`): sets the key cookie for the right
  code; `403` for a wrong one, `429` when throttled, `404` with no
  `LETTER_PIN`. Same-origin only.
- `GET /healthz`: liveness/readiness probe.

## Development

```sh
devenv shell                      # Go, gofumpt, golines, air; sample LETTER_* set
go run . serve                    # http://127.0.0.1:8080
LETTER_AI=false go run . serve    # canned notes only
```

Checks (CI runs the same):

```sh
go vet ./... && go test -race ./... && gofumpt -d . && golines --max-len=160 . --dry-run
nix-build nix/oci.nix --argstr doCheck false   # when go.mod or nix/ change
```

## Building and deploying

- `Dockerfile` and `nix/oci.nix` both build the image (with Claude Code and a
  shell for signing in).
- CI (`.github/workflows/image.yaml`) pushes an image tagged with the
  commit's 8-character SHA, or the git tag, on every push to `main`.
- `workload.yaml` describes the workload for OpenChoreo: one HTTP endpoint
  on 8080. Set the words per environment as env vars, and `LETTER_PIN` (and
  `CLAUDE_CODE_OAUTH_TOKEN`, if used) from secrets.
