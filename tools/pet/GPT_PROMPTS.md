# 桌寵素材:給 GPT 生圖的提示詞

## 做法

1. 開一個新的 ChatGPT 對話,**先上傳參考圖**(角色長相以它為準):
   - 魔女裝:`tools/pet/base/witch.png`
   - 旗袍:`tools/pet/base/qipao.png`
   - 和服:`tools/pet/base/kimono.png`
   (一套衣服開一個對話,上傳那一套的底圖。)
2. 第一則訊息貼「角色設定」(下面那段,把【衣服】換成該套的描述)。
3. 之後**一則訊息只要一張**,貼「動作」那幾段,一次一個。
4. 每張存成 `<衣服>_<動作>.png`,例如 `witch_walk.png`、`qipao_sleep.png`,
   全部放到 `tools/pet/gpt/`,跟我說一聲,我會去背、裁切、統一大小、裝進 LiriOS。
   (有透明背景最好;沒有也沒關係,我這邊會自動去背。)

每張圖拿到後先看一眼:**兩隻手、兩隻腳、每隻手五根手指**。不對就回「手（或腳）多了一隻，重畫，其他不變」。

---

## 角色設定(第一則訊息)

```
I'm making a set of desktop-pet sprites. Use the attached image as the exact character reference.

Character (keep identical in every image):
- Chibi anime girl, 2.5 heads tall, big head, small body
- Long straight navy-blue hair reaching her waist, blunt bangs
- Two small gold star hairpins on the left and right of her bangs
- Large golden/amber eyes, soft pink blush
- Clean cel shading, crisp dark outline, same art style as the reference

Outfit: 【OUTFIT】

Rules for EVERY image I ask for:
- Exactly one character, full body, nothing cut off
- Anatomy must be correct: exactly two arms, two hands with five fingers each, exactly two legs and two feet
- Transparent background (PNG); if transparency is impossible, plain pure white background
- No ground shadow, no props on the ground, no text, no border, no extra effects
- Square 1024×1024, character centered and filling about 85% of the image height
- Same scale, same character, same outfit in every image

Reply "ready" and wait for my pose requests.
```

把【OUTFIT】換成:

- **魔女裝(witch)**
  `small dark-purple witch hat with a black band; short dark-purple cape with gold crescent-moon and star pattern tied with a black ribbon bow; white frilly blouse; white ruffled skirt; white socks; black Mary Jane shoes; holds a small wand with a glowing gold star in one hand`
- **旗袍(qipao)**
  `no hat; short sky-blue cheongsam (qipao) dress with silver crescent-moon embroidery, mandarin collar and silver frog buttons, short sleeves; white flat shoes; tiny star tassel earrings`
- **和服(kimono)**
  `no hat; deep indigo furisode kimono with small gold stars and crescent-moon pattern, long wide sleeves; lavender obi sash tied in a big bow; white tabi socks and wooden geta sandals; holds a small folding fan in one hand`

---

## 動作(一則一張)

**stand(站著)**
```
Pose "stand": standing upright facing the viewer, arms relaxed at her sides (or holding her item), gentle closed-mouth smile.
```

**blink(眨眼,做眨眼動畫用)**
```
Pose "blink": exactly the same as "stand" — identical body, arms, legs, framing — but with both eyes closed in a relaxed way. Only the eyes change.
```

**walk(走路)**
```
Pose "walk": walking toward the right side of the image, three-quarter view, mid-stride with one leg forward and one back, arms swinging naturally, cheerful smile. Two legs only.
```

**walksheet(走路循環,4 格一張圖)**——走路動畫靠這張,比單張 walk 順很多
```
Pose "walksheet": a walk-cycle sprite sheet in ONE image — 4 frames of the same character side by side in 4 equal columns, left to right, all walking toward the right (side view):
1. contact: left leg forward, right leg back, heel touching the ground
2. passing: legs passing each other under the body, body slightly higher
3. contact: right leg forward, left leg back
4. passing: legs passing again, body slightly higher
Identical character, outfit, size and scale in all 4 frames, feet on the same baseline, arms swinging opposite to the legs. Exactly two arms and two legs in each frame, no overlap between frames, transparent background, wide image (e.g. 2048×1024).
```

**sit(坐著)**
```
Pose "sit": sitting on the ground facing the viewer, knees together and legs folded to one side, both hands resting on her lap, calm smile. Both feet visible on the same side, two legs only.
```

**sleep(睡覺)**
```
Pose "sleep": lying on her side on the ground, sleeping peacefully with eyes closed, one hand under her cheek and the other resting in front of her, legs slightly bent. Horizontal composition, two arms and two legs only.
```

**happy(開心,被點時)**
```
Pose "happy": jumping with joy, both arms raised high above her head with open hands, eyes closed in happy upward curves, big open-mouthed smile, both feet slightly off the ground.
```

**surprised(驚訝)**
```
Pose "surprised": standing, startled — wide eyes, small round open mouth, both hands raised near her chest with open palms, leaning back slightly.
```

**held(被拎起來)**
```
Pose "held": being picked up — floating in the air with both arms raised straight up above her head, legs hanging loosely, feet off the ground, flustered blushing pout. Nothing holding her is visible.
```

**dance(跳舞,有音樂時)**
```
Pose "dance": dancing happily — standing on her left foot with her right knee lifted, arms spread out to the sides, winking with a smile. Exactly two legs: one on the ground, one lifted.
```

---

## 檔名對照

| 動作 | 檔名 |
|---|---|
| 站著 | `<衣服>_stand.png` |
| 眨眼 | `<衣服>_blink.png` |
| 走路 | `<衣服>_walk.png` |
| 走路循環(4 格) | `<衣服>_walksheet.png` |
| 坐著 | `<衣服>_sit.png` |
| 睡覺 | `<衣服>_sleep.png` |
| 開心 | `<衣服>_happy.png` |
| 驚訝 | `<衣服>_surprised.png` |
| 被拎起來 | `<衣服>_held.png` |
| 跳舞 | `<衣服>_dance.png` |

`<衣服>` 是 `witch`、`qipao`、`kimono`。三套 × 10 張 = 30 張;先做一套也可以,沒有的動作桌寵會先用站姿頂著。
