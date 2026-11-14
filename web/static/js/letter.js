// Opens the envelope, asks the question and streams the note Claude writes
// after each answer. No inline scripts or styles (strict CSP): data comes
// from data-* attributes, styles from classes and CSS custom properties.
(() => {
    "use strict";

    const stage = document.querySelector(".stage");
    if (!stage) return;
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const $ = (sel, root = document) => root.querySelector(sel);
    // Set when the code was just entered, so the letter opens right away
    // after the reload instead of asking for a second tap on the seal.
    const UNSEALED = "letter-unsealed";

    // The code (LETTER_PIN): only the envelope is served until it's entered.

    if (stage.classList.contains("is-locked")) {
        const envelope = $(".envelope");
        const form = $(".lock");
        const input = $(".lock-pin");
        const hint = $(".lock-hint");
        const buttons = [...document.querySelectorAll(".seal, .lock-open")];
        let trying = false;
        const say = (text, cls) => {
            hint.textContent = text;
            hint.classList.toggle("is-wrong", cls === "wrong");
        };
        const shake = () => {
            if (calm) return;
            envelope.classList.remove("is-shaking");
            void envelope.offsetWidth;
            envelope.classList.add("is-shaking");
        };
        form.addEventListener("submit", async (e) => {
            e.preventDefault();
            const pin = input.value.trim();
            if (trying) return;
            if (!pin) {
                input.focus();
                shake();
                return;
            }
            trying = true;
            buttons.forEach((b) => (b.disabled = true));
            let res;
            try {
                res = await fetch("/api/unlock", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ pin }),
                });
            } catch {
                res = null;
            }
            if (res && res.ok) {
                try {
                    sessionStorage.setItem(UNSEALED, "1");
                } catch {}
                envelope.classList.add("is-opening");
                setTimeout(() => location.reload(), calm ? 0 : 900);
                return;
            }
            trying = false;
            buttons.forEach((b) => (b.disabled = false));
            if (res && res.status === 429) {
                say("let’s take a breath, try again in a minute");
            } else if (res && res.status === 403) {
                say("not quite, try again", "wrong");
                input.select();
            } else {
                say("the post is slow, try again");
            }
            shake();
        });
        $(".envelope-art").addEventListener("click", (e) => {
            if (!e.target.closest(".seal")) input.focus();
        });
        return;
    }

    let labels = [];
    try {
        labels = JSON.parse(stage.dataset.noLabels || "[]");
    } catch {
        labels = [];
    }

    const envelope = $(".envelope");
    const seal = $(".seal");
    const letter = $(".letter");
    const ask = $(".ask");
    const yesBtn = $(".answer-yes");
    const noBtn = $(".answer-no");
    const notesEl = $(".notes");
    const yesCard = $(".yes");
    const drops = [...document.querySelectorAll(".drops path")];

    let noes = 0;
    let busy = false;
    const notes = [];

    // The envelope

    function openLetter() {
        if (envelope.classList.contains("is-opening")) return;
        envelope.classList.add("is-opening");
        setTimeout(
            () => {
                envelope.hidden = true;
                letter.hidden = false;
                requestAnimationFrame(() => letter.classList.add("is-shown"));
                $(".letter-title").focus({ preventScroll: true });
            },
            calm ? 0 : 1100,
        );
    }
    seal.addEventListener("click", openLetter);
    let unsealed = false;
    try {
        unsealed = sessionStorage.getItem(UNSEALED) === "1";
        sessionStorage.removeItem(UNSEALED);
    } catch {}
    if (unsealed) {
        envelope.hidden = true;
        letter.hidden = false;
        requestAnimationFrame(() => letter.classList.add("is-shown"));
    }
    $(".envelope-art").addEventListener("click", (e) => {
        if (e.target !== seal && !seal.contains(e.target)) openLetter();
    });

    // Streaming a note

    // write posts the answer and calls onText with the note so far, then
    // resolves with the last event ({done, gift…} or {error}).
    async function write(answer, onText) {
        const res = await fetch("/api/answer", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ answer, noes, notes: notes.slice(-12) }),
        });
        if (!res.ok || !res.body) return { error: res.status === 429 ? "busy" : "failed" };
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        let text = "";
        let last = { error: "failed" };
        for (;;) {
            const { value, done } = await reader.read();
            if (done) break;
            buf += decoder.decode(value, { stream: true });
            let nl;
            while ((nl = buf.indexOf("\n")) >= 0) {
                const line = buf.slice(0, nl).trim();
                buf = buf.slice(nl + 1);
                if (!line) continue;
                let ev;
                try {
                    ev = JSON.parse(line);
                } catch {
                    continue;
                }
                if (ev.text) {
                    text += ev.text;
                    onText(text);
                }
                if (ev.error) last = { ...ev, text };
                if (ev.done) last = { ...ev, text, error: last.error === "away" ? "away" : undefined };
            }
        }
        return last;
    }

    function setBusy(on) {
        busy = on;
        yesBtn.disabled = on;
        noBtn.disabled = on;
        ask.classList.toggle("is-busy", on);
    }

    // The no

    function sulk() {
        // A little more rain each time, and the buttons lean towards yes.
        drops.forEach((d, i) => d.classList.toggle("on", i < noes));
        stage.style.setProperty("--gloom", String(Math.min(noes / 6, 1)));
        stage.style.setProperty("--yes-scale", String(1 + Math.min(noes, 6) * 0.08));
        stage.style.setProperty("--no-scale", String(1 - Math.min(noes, 6) * 0.05));
        if (noes >= labels.length) {
            noBtn.classList.add("is-leaving");
            setTimeout(() => (noBtn.hidden = true), calm ? 0 : 500);
        } else {
            noBtn.textContent = labels[noes];
        }
        if (!calm) {
            noBtn.classList.remove("wobble");
            void noBtn.offsetWidth;
            noBtn.classList.add("wobble");
        }
    }

    async function sayNo() {
        if (busy) return;
        setBusy(true);
        notesEl.querySelectorAll(".note").forEach((n, i, all) => n.classList.toggle("is-old", i < all.length - 1));
        const li = document.createElement("li");
        li.className = "note is-writing";
        li.style.setProperty("--tilt", `${(Math.random() * 3 - 1.5).toFixed(2)}deg`);
        const ink = document.createElement("span");
        ink.className = "ink";
        li.append(ink);
        notesEl.append(li);
        // Only the last three stay on the page.
        while (notesEl.children.length > 3) notesEl.firstElementChild.remove();
        li.scrollIntoView({ block: "nearest", behavior: calm ? "auto" : "smooth" });

        let ev;
        try {
            ev = await write("no", (t) => (ink.textContent = t));
        } catch {
            ev = { error: "failed" };
        }
        li.classList.remove("is-writing");
        if (ev.error === "busy" || (ev.error === "failed" && !ev.text)) {
            ink.textContent = "One moment, I’m still writing the last one…";
            li.classList.add("is-hint");
            setBusy(false);
            return;
        }
        if (ev.text) notes.push(ev.text);
        noes++;
        sulk();
        setBusy(false);
    }

    // The yes

    const promise = /^\s*[—–\-•*]\s+/;

    function render(text, note, list, final) {
        const lines = text.split("\n");
        const said = [];
        const vows = [];
        lines.forEach((l, i) => {
            if (promise.test(l)) vows.push(l.replace(promise, ""));
            else if (l.trim() && !(i === lines.length - 1 && !final && /^\s*[—–\-•*]?\s*$/.test(l))) said.push(l.trim());
        });
        note.textContent = said.join(" ");
        while (list.children.length > vows.length) list.lastElementChild.remove();
        vows.forEach((v, i) => {
            let li = list.children[i];
            if (!li) {
                li = document.createElement("li");
                const b = document.createElement("button");
                b.type = "button";
                b.className = "promise";
                b.disabled = true;
                li.append(b);
                list.append(li);
            }
            li.firstElementChild.textContent = v;
        });
        return vows.length;
    }

    function showGift(ev) {
        const gift = $(".gift");
        if (!gift || !ev.gift) return;
        $(".gift-intro", gift).textContent = ev.intro || "";
        $(".gift-value", gift).textContent = ev.gift;
        const open = $(".gift-open", gift);
        if (ev.link) {
            open.href = ev.gift;
            open.hidden = false;
        }
        gift.hidden = false;
        requestAnimationFrame(() => gift.classList.add("is-shown"));
        const code = $(".gift-code", gift);
        code.addEventListener("click", async () => {
            if (code.classList.contains("is-hidden")) {
                code.classList.remove("is-hidden");
                $(".gift-hint", code).textContent = "tap to copy";
                return;
            }
            try {
                await navigator.clipboard.writeText(ev.gift);
                $(".gift-hint", code).textContent = "copied";
            } catch {
                // No clipboard: it's on screen anyway.
            }
        });
    }

    async function sayYes() {
        if (busy) return;
        setBusy(true);
        stage.classList.add("is-yes");
        const r = yesBtn.getBoundingClientRect();
        celebrate(r.left + r.width / 2, r.top + r.height / 2);
        setTimeout(
            () => {
                ask.hidden = true;
                yesCard.hidden = false;
                requestAnimationFrame(() => yesCard.classList.add("is-shown"));
                $(".yes-title").focus({ preventScroll: true });
                yesCard.scrollIntoView({ block: "start", behavior: calm ? "auto" : "smooth" });
            },
            calm ? 0 : 700,
        );

        const noteP = $(".yes-note");
        const ink = $(".ink", noteP);
        const list = $(".promises");
        let ev;
        try {
            ev = await write("yes", (t) => render(t, ink, list, false));
        } catch {
            ev = { error: "failed" };
        }
        noteP.classList.remove("is-writing");
        const vows = ev.text ? render(ev.text, ink, list, true) : 0;
        if (!ev.text) ink.textContent = "Thank you. Truly.";
        if (vows === 0) {
            showGift(ev);
            return;
        }
        // Accept each promise; the gift waits for the last one.
        $(".promises-hint").hidden = false;
        let left = vows;
        list.querySelectorAll(".promise").forEach((b) => {
            b.disabled = false;
            b.addEventListener(
                "click",
                () => {
                    b.classList.add("is-kept");
                    b.disabled = true;
                    b.setAttribute("aria-label", `${b.textContent} (accepted)`);
                    const br = b.getBoundingClientRect();
                    celebrate(br.left + 18, br.top + br.height / 2, 14);
                    if (--left === 0) {
                        $(".promises-hint").textContent = "sealed";
                        showGift(ev);
                    }
                },
                { once: true },
            );
        });
    }

    yesBtn.addEventListener("click", sayYes);
    noBtn.addEventListener("click", sayNo);

    // Petals

    const canvas = $(".confetti");
    const ctx = canvas && canvas.getContext("2d");
    const css = getComputedStyle(document.body);
    const palette = ["--accent", "--accent-soft", "--gold", "--blush"].map((v) => css.getPropertyValue(v).trim() || "#c77");
    let pieces = [];
    let frame = 0;

    function resize() {
        const dpr = window.devicePixelRatio || 1;
        canvas.width = innerWidth * dpr;
        canvas.height = innerHeight * dpr;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    function celebrate(x, y, count = 70) {
        if (!ctx || calm) return;
        for (let i = 0; i < count; i++) {
            const a = -Math.PI / 2 + (Math.random() - 0.5) * Math.PI * 1.1;
            const v = 3 + Math.random() * 6;
            pieces.push({
                x,
                y,
                vx: Math.cos(a) * v,
                vy: Math.sin(a) * v,
                r: 3 + Math.random() * 4,
                spin: Math.random() * Math.PI,
                vs: (Math.random() - 0.5) * 0.2,
                heart: Math.random() < 0.35,
                color: palette[i % palette.length],
                life: 1,
            });
        }
        if (!frame) frame = requestAnimationFrame(draw);
    }

    function heart(r) {
        ctx.beginPath();
        ctx.moveTo(0, r * 0.35);
        ctx.bezierCurveTo(-r * 1.2, -r * 0.5, -r * 0.4, -r * 1.2, 0, -r * 0.45);
        ctx.bezierCurveTo(r * 0.4, -r * 1.2, r * 1.2, -r * 0.5, 0, r * 0.35);
        ctx.fill();
    }

    function draw() {
        ctx.clearRect(0, 0, innerWidth, innerHeight);
        pieces = pieces.filter((p) => p.life > 0 && p.y < innerHeight + 20);
        for (const p of pieces) {
            p.vy += 0.12;
            p.vx *= 0.985;
            p.x += p.vx + Math.sin(p.spin) * 0.6;
            p.y += p.vy;
            p.spin += p.vs;
            p.life -= 0.006;
            ctx.save();
            ctx.globalAlpha = Math.max(p.life, 0);
            ctx.translate(p.x, p.y);
            ctx.rotate(p.spin);
            ctx.fillStyle = p.color;
            if (p.heart) heart(p.r * 1.3);
            else {
                ctx.beginPath();
                ctx.ellipse(0, 0, p.r, p.r * 0.55, 0, 0, Math.PI * 2);
                ctx.fill();
            }
            ctx.restore();
        }
        frame = pieces.length ? requestAnimationFrame(draw) : 0;
        if (!frame) ctx.clearRect(0, 0, innerWidth, innerHeight);
    }

    if (ctx) {
        resize();
        addEventListener("resize", resize);
    }
})();
