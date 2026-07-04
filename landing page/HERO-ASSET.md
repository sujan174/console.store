# consolestore — cinematic hero video brief

Full-bleed looping background video for the landing hero — **warm, cozy, late
night** (welcoming like zo.house, but authentically consolestore: terminal +
food + night). Drop the finished files here and it auto-plays; no code change:

```
landing page/public/hero.mp4          # required (H.264)
landing page/public/hero.webm         # optional (smaller; VP9/AV1)
landing page/public/hero-poster.jpg   # required (first frame, static fallback)
```

Until they exist the hero falls back to the starfield automatically. Under
reduced-motion the poster shows paused (never black).

---

## Concept — "cozy late-night desk"

A warm, intimate late-night scene: a developer's desk lit by a warm desk lamp.
A laptop / monitor glows softly with an open terminal (kept **out of focus** so
no text is readable — just the warm-and-blue glow of a live terminal). Beside it,
a **bowl of ramen or a coffee mug gently steaming**. Through a dark window
behind, soft blurred **city lights bokeh** at night. Shallow depth of field,
warm amber tones lifted by the cool blue glow of the screen. Calm, inviting,
"order dinner from your terminal at 1am." Slow subtle motion — steam curling,
bokeh twinkling, a faint push-in.

### Alternates (same cozy-night feeling)
- **Over-the-shoulder** — softly framed from behind, terminal glow + steaming bowl, city window beyond.
- **Top-down desk flatlay** — warm-lit desk from above: keyboard, glowing screen edge, steaming bowl, slow drift.

---

## Composition rules (critical — text sits on top)

- **Keep the upper-center dark and calm** — the wordmark "console store" sits
  center; put the dark window / wall + soft bokeh there. The warm props (lamp,
  bowl, keyboard, screen glow) live in the **lower third and the sides**.
- Nav rides the top, the install command the bottom — keep those bands quiet.
- 16:9, horizontal. We `object-fit: cover`, so leave safe margins.
- **No readable text on the screen, no logos, no brand marks, no UI, no
  watermark** — the terminal glow must stay blurred/abstract; we overlay our own.
- Slow, continuous motion (steam + gentle push-in). **No cuts, no shake, no flashing.**
- **Seamless loop** — last frame blends into the first (8–14s). Steam/bokeh that loops cleanly.

## Palette (warm night, Tokyo-Night-compatible)
- warm amber lamp light + honey tones (hero warmth)
- cool terminal glow: blue `#93a8ff` / cyan `#7fe0ff`, a touch of gold `#eab560`
- deep near-black surroundings (`#0b0b14`) so it stays moody and premium.

---

## Copy-paste prompt — text-to-video (Sora / Veo / Runway / Kling / Luma / Pika)

> Cinematic, intimate close-up of a cozy developer's desk late at night, lit by a
> warm desk lamp. A laptop screen glows softly with a live terminal, blurred and
> out of focus so no text is readable — just warm amber and cool blue light. Next
> to it a bowl of ramen and a coffee mug gently steam, wisps of steam curling
> slowly upward. Through a dark window behind, soft out-of-focus city lights
> bokeh twinkle at night. Shallow depth of field, warm honey-amber tones lifted
> by the cool blue glow of the screen, deep near-black shadows, moody and
> premium. Calm, inviting, hopeful — the feeling of ordering dinner from your
> terminal at 1am. Extremely slow, smooth camera push-in; the only motion is
> curling steam and shimmering bokeh. The upper-center is dark, quiet window and
> wall with room for a title; the warm props sit low and to the sides. Cinematic,
> photographic, dreamy, seamless loop.

**Negative / avoid:** readable text, terminal words, code you can read, logos,
brand marks, UI, watermark, daylight, bright center, harsh light, people faces,
fast motion, camera shake, hard cuts, flashing, clutter, cartoonish, low-res.

## Copy-paste prompt — still image (Midjourney/DALL-E), for the image→video route

> Cozy developer's desk late at night, warm desk lamp glow, a laptop screen
> softly glowing with a blurred out-of-focus terminal, a bowl of ramen and a
> coffee mug gently steaming, soft out-of-focus city lights bokeh through a dark
> window behind, shallow depth of field, warm amber tones lifted by cool blue
> screen glow, deep near-black shadows, moody premium cinematic photography,
> calm and inviting, dark quiet upper center for negative space --ar 16:9
> --style raw

Then animate the still in Runway / Kling / Luma with a "very slow push-in,
curling steam, shimmering bokeh, seamless loop" motion prompt.

---

## Deliver whatever you've got — I'll optimize

Hand me the raw export (mp4, MOV/ProRes, or even a PNG sequence) at the highest
quality. I'll transcode to web-optimized `hero.mp4` (+ `hero.webm`) and cut the
`hero-poster.jpg` with ffmpeg. Targets: 1080p (or 1440p), H.264 yuv420p, no
audio, ~4–8 MB, seamless loop.

Ideal source: **1920×1080 or higher, 8–14s, no audio, seamless loop.**
