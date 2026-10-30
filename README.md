# DesktopPet

macOS 桌面寵物:Q 版魔女,有旗袍、和服可以換。會在視窗頂邊走動、坐下、睡覺,被拖起來會有反應,滑鼠在面前晃久了才會轉頭看你。

## 需求

macOS 26 以上、Swift 6.2。

## 使用

```bash
swift run DesktopPetApp
swift test
```

右鍵桌寵有選單。

也可以當函式庫用。提供 `isMusicPlaying`,有音樂時她會跳舞:

```swift
import DesktopPet

PetController.shared.isMusicPlaying = { player.isPlaying }
PetController.shared.restore()
```

## 素材

圖片在 `Sources/DesktopPet/Resources/Pet/<衣服>/<動作>.png`,用 AI 生圖工具產生。提示詞在 `tools/pet/GPT_PROMPTS.md`;把新圖放到 `tools/pet/gpt/` 再跑 `tools/pet/import_gpt.py` 就會整理好。
