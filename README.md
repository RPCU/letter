# ✉️ Letter

A little page holding a sealed letter and one question. By default it's an
apology: *"I'm sorry … Do you forgive me?"*. Every word can be changed, so the
same page can wish someone a good trip, ask them to dinner, or say thank you.

1. An envelope with a wax seal (`LETTER_SEAL`, 🥺). Tap the seal to open it.
2. The letter unfolds: date, title, *Dear {name},* your paragraphs and your
   signature.
3. The question, with a little cloud over the sun, and two buttons.
4. **No** → Claude writes a short handwritten note in a new form each time:
   a sincere line, then a haiku, a mock-formal petition, a weather forecast of
   your remorse, a recipe for forgiveness, and finally a gracious "take your
   time". Each No adds rain to the cloud, shrinks No a little and grows Yes.
   Once the No labels run out, the No button slips away and only Yes is left.
5. **Yes** → the cloud drifts off, petals and hearts fly, a *forgiven* stamp
   lands, and Claude writes a thank-you with three small promises built from
   what happened. Tap each promise to accept it. The last one reveals the
   optional gift (`LETTER_GIFT`): blurred until tapped, then tap to copy.

There is no prompt box. Claude only reacts to the two buttons, and it's told
to stay warm, a little witty and respectful: a No is always a valid answer,
with no guilt-tripping and no pressure.

It's one static Go binary (standard library only, with the template, assets
and time zone database embedded), configured entirely through environment
variables. The notes are written by Claude through Claude Code on a Claude
login (no API key), which the image ships. Without it, canned notes stand
in, so the page always works.

## Configuration

Every setting is optional. Multi-line values: in `LETTER_MESSAGE` blank lines
separate paragraphs, and the list settings take one item per line.

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
| `LETTER_PIN`          |                                | A code that locks the page (4 to 64 characters, digits bring up the number pad). See *Locking it* below. |
| `LETTER_THEME`        | `rose`                         | `rose`, `sage`, `ocean`, `dusk` or `gold`. |
| `LETTER_TIMEZONE`     | `UTC`                          | IANA zone for the letter's date and the answers log. |
| `LETTER_AI`           | `true`                         | Claude writes the notes; `false` uses only the canned ones. |
| `LETTER_CONTEXT`      |                                | What happened, for Claude only, never shown: *"I forgot our dinner on Friday"* (max 2000). |
| `LETTER_PROMPT`       |                                | More instructions on how to write: in-jokes, nicknames, things to avoid (max 8000). |
| `LETTER_NO_STYLES`    | six forms (sincere → haiku → petition → forecast → recipe → gracious) | The form of the note after each No, one per line; the last one stays. `{name}` becomes `LETTER_NAME`. |
| `LETTER_YES_STYLE`    | thank-you + three promises     | The form of the note after the yes. Lines starting with `— ` become the promises to accept. |
| `LETTER_FALLBACK_NO`  | six canned notes               | Used when Claude can't write, one per No in turn; the last one stays. |
| `LETTER_FALLBACK_YES` | thank-you + three promises     | Used when Claude can't write the yes note. |
| `LETTER_ANSWERS`      | `true`                         | Log every answer for you (see below). |
| `LETTER_MODEL`        | `sonnet`                       | Claude model (`sonnet`, `opus`, `haiku` or a full model ID). |
| `LETTER_COMMAND`      | `claude`                       | The Claude Code CLI. |
| `LETTER_HOST`         | all interfaces                 | Listen address. |
| `LETTER_PORT`         | `8080`                         | Listen port. |

An invalid value stops the server at startup and lists every problem.

### Example: a good trip to Singapore

```yaml
LETTER_NAME: Andie
LETTER_FROM: Victor
LETTER_TITLE: Bon voyage
LETTER_MESSAGE: |
  Two weeks of Singapore: hawker centres, the Gardens by the Bay at night,
  and that humid air you secretly missed.

  Have the most wonderful trip. Eat everything, sleep in a little, and come
  back with stories.
LETTER_QUESTION: Will you send me a photo from Marina Bay?
LETTER_YES: Promise
LETTER_NO: |
  Nope
  Too busy
  Maybe later
LETTER_YES_TITLE: Safe travels
LETTER_STAMP: bon voyage
LETTER_SEAL: ✈️
LETTER_THEME: ocean
LETTER_TIMEZONE: Asia/Singapore
LETTER_CONTEXT: She lived in Singapore before and is going back for two weeks of holiday.
LETTER_NO_STYLES: |
  a playful one-line bribe involving kaya toast
  a haiku about a postcard that never arrives
  a gracious note: no photo needed, just come back safe and happy
LETTER_YES_STYLE: >-
  a warm, happy send-off of one or two sentences, then exactly three little
  wishes for the trip, each on its own line starting with "— "
LETTER_FALLBACK_NO: |
  I'll trade you a kaya toast for one photo. Final offer.
  A postcard drifts by, / lost somewhere over the bay. / I'll still wait for it.
  No photo needed. Just come back happy.
LETTER_FALLBACK_YES: |
  Deal! Have the best time, and eat a satay for me.
  — Warm nights and cool hawker drinks.
  — Every MRT on time.
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
code separately from the link.

## Claude

Each answer runs a fresh `claude --print` in an empty directory, cut off from
the account it runs on: no tools, no MCP servers or claude.ai connectors, no
plugins, skills, hooks, settings, `CLAUDE.md` or memory, no saved session,
its own system prompt instead of Claude Code's, and none of the `LETTER_*`
variables. It knows the letter, the question, the answer, how many times they
said no, the notes already on the page (so it never repeats itself),
`LETTER_CONTEXT` and `LETTER_PROMPT`, and never the gift.

Notes stream in. Answers are throttled across every visitor (12 in a burst,
then 6 a minute, at most 2 notes written at once). Claude Code runs take
turns on a lock file next to the login (`$CLAUDE_CONFIG_DIR/letter-claude.lock`),
so replicas sharing the volume never renew the login at once (two renewals at
once can sign it out). When Claude fails (not signed in, missing, out of
usage), the canned notes stand in after a short pause, a warning is logged,
and Claude is tried again a minute later.

**Signing in.** The image has Claude Code and a shell. Its login lives in
`CLAUDE_CONFIG_DIR` (`/home/letter/.claude`):

```sh
kubectl exec -it <pod> -- claude auth login
```

Mount a volume on `/home/letter/.claude` to keep it across restarts (hestia's
`persistent-volume` trait, `claude` instance); several replicas need a
ReadWriteMany one. Or set `CLAUDE_CODE_OAUTH_TOKEN` (from
`claude setup-token`) from a secret. Each note runs a Claude Code process of
about 250 MiB, two at most at once: give the container ~640 MiB and 2 CPUs.

**Their answers.** Every answer is added to
`$CLAUDE_CONFIG_DIR/letter-answers.md`, next to the login, with the time:

```
- 2026-11-14 21:30 CET: no ("No", no number 1)
- 2026-11-14 21:31 CET: **yes** ("Yes", after 1 no)
```

It's never cleared and never served. Read it with:

```sh
kubectl exec <pod> -- cat /home/letter/.claude/letter-answers.md
```

## Endpoints

- `GET /`: the page.
- `POST /api/answer` (`{"answer": "yes"|"no", "noes": n, "notes": ["…"]}`):
  the note that follows, streamed as JSON lines (`{"text"}`…, then
  `{"done": true}`, with `gift`, `link` and `intro` after a yes when there's a
  gift; or `{"error": "away"}` if Claude stops mid-note). Same-origin only.
  `409` for a No once the No labels have run out, `429` when throttled.
- `POST /api/unlock` (`{"pin": "…"}`): sets the key cookie for the right
  code; `403` for a wrong one, `429` when throttled, `404` with no
  `LETTER_PIN`. Same-origin only. With a code set, `POST /api/answer`
  answers `403` without the cookie.
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

## Deployment

Same flow as birthday's:

1. A push to `main` runs `.github/workflows/image.yaml`, which pushes
   `zot.rpcu.io/public/letter:<8-char SHA>` (or `:<tag>` for tags).
2. The GitHub webhook triggers OpenChoreo's `prebuilt-image` workflow (hestia,
   `openchoreo/namespaces/rpcu/projects/testing/letter.yaml`), which waits
   for that tag and publishes the workload described by `workload.yaml`
   (one HTTP endpoint on 8080).
3. The words are set per environment in hestia's ReleaseBindings (in the
   portal for production and public).
