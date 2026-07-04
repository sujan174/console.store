# consolestore — cinematic hero video brief

Full-bleed looping background video for the landing hero — **warm, welcoming,
sunlit** (zo.house energy). Drop the finished files here and it auto-plays; no
code change needed:

```
landing page/public/hero.mp4          # required (H.264)
landing page/public/hero.webm         # optional (smaller; VP9/AV1)
landing page/public/hero-poster.jpg   # required (first frame, static fallback)
```

Until they exist the hero falls back to the starfield automatically. Under
reduced-motion the poster shows paused (never black).

---

## Concept — "a warm civilization at golden hour"

A magnificent, sunlit fantasy metropolis seen from a slow, gentle aerial drift.
Golden-hour light floods soft sandstone-and-brass architecture — arches, domes,
terraces, hanging gardens. Airships and birds drift lazily across a warm hazy
sky. The mood is **inviting, hopeful, luxurious, a sense of homecoming** — an
epic world that still feels cozy and welcoming, not cold or dystopian.

Warm amber/gold is the hero color (it's our `store` accent, `#eab560`), lifted
by soft blues in the sky. Painterly concept-art finish, cinematic, dreamlike.

### Alternates (same warm feeling)
- **Sunlit harbor city** — a golden coastal town waking at dawn, boats, warm mist, soft glow.
- **Floating garden isles** — warm green-and-gold islands drifting in a hazy amber sky.

---

## Composition rules (critical — text sits on top)

- **Keep the upper-center as calm, open, softly-lit sky** — the wordmark
  "console store" sits center; nav rides the top, the install command the
  bottom. Put the rich architecture / detail in the lower two-thirds and the
  sides, with breathing room across the middle.
- 16:9, horizontal. We `object-fit: cover`, so leave safe margins.
- **No text, no logos, no UI, no readable words, no watermark** — we overlay our own.
- Slow, continuous motion (a gentle push-in or drift). **No cuts, no shake, no flashing.**
- **Seamless loop** — last frame blends into the first (8–14s).

## Palette (warm, Tokyo-Night-compatible)
- warm gold `#eab560` (hero tone) · amber sunlight · soft sandstone / ivory / brass
- lifted by soft sky blue `#93a8ff` and a touch of violet `#b08cf5` in the haze
- luminous and warm overall — golden hour, not night.

---

## Copy-paste prompt — text-to-video (Sora / Veo / Runway / Kling / Luma / Pika)

> Cinematic slow aerial drift over a magnificent sunlit fantasy civilization at
> golden hour. Soft sandstone and brass architecture — grand arches, domes,
> terraces and hanging gardens — glowing in warm amber light. Airships and birds
> drift lazily across a warm hazy sky. Gentle golden sunbeams, soft volumetric
> haze, dust motes floating in the light, subtle depth of field. Warm and
> inviting, hopeful, luxurious, a feeling of homecoming and welcome. Color
> palette: warm gold (#eab560), amber and honey light, soft ivory stone, lifted
> by a gentle blue sky. Extremely slow, smooth, continuous camera push-in — calm
> and serene. The upper-center of the frame is open, soft, glowing sky with room
> for a title; the rich detail sits below and to the sides. Painterly concept-art
> style, dreamlike, epic yet cozy, seamless loop.

**Negative / avoid:** text, letters you can read, logos, UI, watermark, night,
darkness, neon, cyberpunk, dystopian, cold blue tones, people close-up, fast
motion, camera shake, hard cuts, flashing, lens flares, cartoonish, low-res.

## Copy-paste prompt — still image (Midjourney/DALL-E), for the image→video route

> A magnificent sunlit fantasy civilization at golden hour, warm sandstone and
> brass architecture with grand arches and domes and hanging gardens, airships
> drifting in a warm hazy sky, golden sunbeams and soft volumetric haze, inviting
> and hopeful and luxurious, warm gold and amber and honey palette lifted by soft
> blue sky, open glowing sky in the upper center for negative space, painterly
> cinematic concept art, ultra detailed, dreamlike, epic yet cozy --ar 16:9
> --style raw

Then animate the still in Runway / Kling / Luma with a "very slow gentle
push-in, drifting airships, floating dust motes, seamless loop" motion prompt.

---

## Deliver whatever you've got — I'll optimize

Hand me the raw export (mp4, MOV/ProRes, or even a PNG sequence) at the highest
quality. I'll transcode to web-optimized `hero.mp4` (+ `hero.webm`) and cut the
`hero-poster.jpg` with ffmpeg. Targets I'll hit: 1080p (or 1440p), H.264
yuv420p, no audio, ~4–8 MB, seamless loop.

Ideal source: **1920×1080 or higher, 8–14s, no audio, seamless loop.**

> Note: on warm/bright footage the centered wordmark may need a touch more
> backing — I'll tune the scrim / add a soft radial behind the wordmark once the
> real asset lands so "console store" stays crisp.
