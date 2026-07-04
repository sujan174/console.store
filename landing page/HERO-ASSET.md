# consolestore — cinematic hero video brief

Full-bleed looping background video for the landing hero (zo.house energy).
Drop the finished files here and it auto-plays; no code change needed:

```
landing page/public/hero.mp4          # required (H.264)
landing page/public/hero.webm         # optional (smaller; VP9/AV1)
landing page/public/hero-poster.jpg   # required (first frame, static fallback)
```

Until they exist the hero falls back to the starfield automatically. Under
reduced-motion the poster shows paused (never black).

---

## Recommended concept — "terminal metropolis at night"

A vast, dark cyber-city seen from a slow aerial drift. The skyscrapers are
built from glowing **monospace glyphs / terminal windows** — countless tiny lit
characters, like a city made of code. Rivers of light data flow between towers.
Tokyo-Night palette: near-black sky, cool blue + violet glow, rare **gold**
beacons. Cinematic, moody, luxe, mysterious — the console.store world.

(This continues the "console city" look from the other branch, but as real
rendered cinematic art instead of canvas.)

### Alternates (if you want a different vibe)
- **Data aurora canyon** — slow flight through a canyon of flowing code + aurora ribbons; abstract, elegant.
- **Console cosmos** — a dark galaxy made of tiny terminal windows/characters, slowly rotating.

---

## Composition rules (critical — text sits on top)

- **Keep the center ~40% and the very top & bottom DARK and low-detail** — the
  wordmark "console store" sits dead center; nav rides the top, the install
  command the bottom. Put the interesting motion/light in the mid-band and edges.
- 16:9, horizontal. We `object-fit: cover`, so leave safe margins.
- **No text, no logos, no UI, no readable words, no watermark** — we overlay our own.
- Slow, continuous motion (a gentle push-in or drift). **No cuts, no shake, no flashing.**
- **Seamless loop** — last frame blends into the first (8–14s).

## Palette (Tokyo Night)
- background `#030307` / `#0b0b14`
- blue `#93a8ff` · violet `#b08cf5` · cyan `#7fe0ff` · gold `#eab560`
- Dark overall, high-contrast pinpoints of light.

---

## Copy-paste prompt — text-to-video (Sora / Veo / Runway / Kling / Luma / Pika)

> Cinematic slow aerial drift over a vast futuristic city at night, entirely
> dark and moody. The skyscrapers are made of glowing monospace terminal
> characters and thousands of tiny lit windows, like a metropolis built from
> code. Rivers of blue and violet light flow between the towers, rare warm gold
> beacons glow in the distance. Tokyo-night color palette: near-black sky, deep
> blues (#93a8ff), soft violet (#b08cf5), gold accents (#eab560). Volumetric
> haze, subtle depth of field, faint drifting embers of light. Extremely slow,
> smooth, continuous camera push-in — calm and hypnotic. The center of the frame
> and the top and bottom edges stay dark and empty; the light and detail live in
> the mid-band and sides. Luxurious, high-end, atmospheric, seamless loop.

**Negative / avoid:** text, letters you can read, logos, UI, watermark, people,
cars, daylight, bright center, fast motion, camera shake, hard cuts, flashing,
lens flares, cartoonish, low-res.

## Copy-paste prompt — still image (Midjourney/DALL-E), for the image→video route

> A vast futuristic metropolis at night built entirely from glowing monospace
> terminal characters and tiny lit windows, city of code, rivers of blue and
> violet light between the towers, rare gold beacons, near-black moody sky,
> volumetric haze, cinematic wide aerial shot, dark empty center for negative
> space, Tokyo-night palette deep blue violet and gold, ultra detailed,
> atmospheric, luxurious --ar 16:9 --style raw

Then animate the still in Runway / Kling / Luma with a "very slow push-in,
subtle drifting lights, seamless loop" motion prompt.

---

## Deliver whatever you've got — I'll optimize

Hand me the raw export (mp4, MOV/ProRes, or even a PNG sequence) at the highest
quality. I'll transcode to web-optimized `hero.mp4` (+ `hero.webm`) and cut the
`hero-poster.jpg` with ffmpeg. Targets I'll hit: 1080p (or 1440p), H.264
yuv420p, no audio, ~4–8 MB, seamless loop.

Ideal source: **1920×1080 or higher, 8–14s, no audio, seamless loop.**
