# kimono:整套提示詞

參考圖:`tools/pet/base/kimono.png`(開新對話先上傳)。由上往下,一則訊息貼一段。

## 0. 角色設定(第一則)

```
I'm making a set of desktop-pet sprites. Use the attached image as the exact character reference.

Character (keep identical in every image):
- Chibi anime girl, 2.5 heads tall, big head, small body
- Long straight navy-blue hair reaching her waist, blunt bangs
- Two small gold star hairpins on the left and right of her bangs
- Large golden/amber eyes, soft pink blush
- Clean cel shading, crisp dark outline, same art style as the reference

Outfit: no hat; deep indigo furisode kimono with small gold stars and crescent-moon pattern, long wide sleeves; lavender obi sash tied in a big bow; white tabi socks and wooden geta sandals; holds a small folding fan in one hand

Rules for EVERY image I ask for:
- Exactly one character, full body, nothing cut off
- Anatomy must be correct: exactly two arms, two hands with five fingers each, exactly two legs and two feet
- Transparent background (PNG); if transparency is impossible, plain pure white background
- No ground shadow, no props on the ground, no text, no border, no extra effects
- Square 1024×1024, character centered and filling about 85% of the image height
- Same scale, same character, same outfit in every image

Reply "ready" and wait for my pose requests.
```

## 1. stand(站著)→ 存成 `kimono_stand.png`

```
Pose "stand": standing upright facing the viewer, arms relaxed at her sides (or holding her item), gentle closed-mouth smile.
```

## 2. blink(眨眼,做眨眼動畫用)→ 存成 `kimono_blink.png`

```
Pose "blink": exactly the same as "stand" — identical body, arms, legs, framing — but with both eyes closed in a relaxed way. Only the eyes change.
```

## 3. walk(走路)→ 存成 `kimono_walk.png`

```
Pose "walk": walking toward the right side of the image, three-quarter view, mid-stride with one leg forward and one back, arms swinging naturally, cheerful smile. Two legs only.
```

## 4. walksheet(走路循環,4 格一張圖)→ 存成 `kimono_walksheet.png`

```
Pose "walksheet": a walk-cycle sprite sheet in ONE image — 4 frames of the same character side by side in 4 equal columns, left to right, all walking toward the right (side view):
1. contact: left leg forward, right leg back, heel touching the ground
2. passing: legs passing each other under the body, body slightly higher
3. contact: right leg forward, left leg back
4. passing: legs passing again, body slightly higher
Identical character, outfit, size and scale in all 4 frames, feet on the same baseline, arms swinging opposite to the legs. Exactly two arms and two legs in each frame, no overlap between frames, transparent background, wide image (e.g. 2048×1024).
```

## 5. sit(坐著)→ 存成 `kimono_sit.png`

```
Pose "sit": sitting on the ground facing the viewer, knees together and legs folded to one side, both hands resting on her lap, calm smile. Both feet visible on the same side, two legs only.
```

## 6. sleep(睡覺)→ 存成 `kimono_sleep.png`

```
Pose "sleep": lying on her side on the ground, sleeping peacefully with eyes closed, one hand under her cheek and the other resting in front of her, legs slightly bent. Horizontal composition, two arms and two legs only.
```

## 7. happy(開心,被點時)→ 存成 `kimono_happy.png`

```
Pose "happy": jumping with joy, both arms raised high above her head with open hands, eyes closed in happy upward curves, big open-mouthed smile, both feet slightly off the ground.
```

## 8. surprised(驚訝)→ 存成 `kimono_surprised.png`

```
Pose "surprised": standing, startled — wide eyes, small round open mouth, both hands raised near her chest with open palms, leaning back slightly.
```

## 9. held(被拎起來)→ 存成 `kimono_held.png`

```
Pose "held": being picked up — floating in the air with both arms raised straight up above her head, legs hanging loosely, feet off the ground, flustered blushing pout. Nothing holding her is visible.
```

## 10. dance(跳舞,有音樂時)→ 存成 `kimono_dance.png`

```
Pose "dance": dancing happily — standing on her left foot with her right knee lifted, arms spread out to the sides, winking with a smile. Exactly two legs: one on the ground, one lifted.
```
