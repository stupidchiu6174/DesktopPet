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

## Windows 版

`windows/` 是移植過去的 Windows 版:Go + Win32,沒有 cgo,在 macOS / Linux 上就能交叉編譯(需要 Go 1.24 以上)。

```bash
windows/build.sh
```

產出 `windows/dist/DesktopPet.exe`(64 位元,Windows 10 / 11)。素材直接編進 exe,單一檔案雙擊就能跑。

- 她會站在工作列上緣、一般視窗的上緣,行為跟 macOS 版相同:拖曳、點一下、點兩下換衣服、右鍵選單。
- 「收起桌寵」之後從通知區域的圖示叫回來(點一下切換,右鍵可以結束)。
- 衣服記在 `%APPDATA%\DesktopPet\settings.json`;把新素材放到 `%APPDATA%\DesktopPet\Pet\<衣服>\<動作>.png` 會優先讀取。
- 跳舞(有音樂時)目前只有 macOS 版有。

## 素材

圖片在 `Sources/DesktopPet/Resources/Pet/<衣服>/<動作>.png`,用 AI 生圖工具產生。提示詞在 `tools/pet/GPT_PROMPTS.md`;把新圖放到 `tools/pet/gpt/` 再跑 `tools/pet/import_gpt.py` 就會整理好。
