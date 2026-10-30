"""把 GPT 生的桌寵素材(透明背景 PNG)整理進 Resources/Pet/<衣服>/<動作>.png。

GPT 每張的比例不一定一致(重畫的常常變小),所以每個動作各有固定的目標高度:
站姿 480px(畫面上 ÷3 = 160pt),舉手的高一點,坐著、躺著矮一點。裁到角色邊界,腳底對齊圖的底部。

來源:tools/pet/gpt/<衣服>_<動作>.png      用法:python3 import_gpt.py
走路循環:tools/pet/gpt/<衣服>_walksheet.png(一張圖裡左到右 4 格)→ walk1…walk4.png。
         4 格用同一個縮放比例,保留每格之間自然的高低起伏。
"""
from __future__ import annotations

from pathlib import Path

from PIL import Image

HERE = Path(__file__).parent
SRC = HERE / "gpt"
OUT = HERE.parent.parent / "Sources" / "DesktopPet" / "Resources" / "Pet"
HEIGHTS = {"stand": 480, "blink": 480, "walk": 475, "surprised": 480, "held": 500,
           "happy": 440, "dance": 470, "sit": 340, "sleep": 200}


def trimmed(path: Path) -> Image.Image:
    image = Image.open(path).convert("RGBA")
    # 幾乎透明的雜點不算邊界
    alpha = image.getchannel("A").point(lambda a: 255 if a > 24 else 0)
    return image.crop(alpha.getbbox())


def main() -> None:
    by_outfit: dict[str, dict[str, Path]] = {}
    for f in sorted(SRC.glob("*.png")):
        outfit, _, pose = f.stem.partition("_")
        by_outfit.setdefault(outfit, {})[pose] = f
    for outfit, poses in by_outfit.items():
        (OUT / outfit).mkdir(parents=True, exist_ok=True)
        if "walksheet" in poses:
            split_walk(outfit, poses.pop("walksheet"))
        for pose, path in poses.items():
            image = trimmed(path)
            # 走路循環的各格(walk1…)跟走路同高
            target = HEIGHTS.get(pose.rstrip("0123456789"), 480)
            scale = target / image.height
            size = (round(image.width * scale), target)
            image.resize(size, Image.LANCZOS).save(OUT / outfit / f"{pose}.png", optimize=True)
            print(f"{outfit}/{pose}.png  {size[0]}×{size[1]}")


def columns(sheet: Image.Image, frames: int) -> list[tuple[int, int]]:
    """依角色之間的透明空白切格(不硬切等分,免得頭髮、裙襬被切掉);切不出剛好的格數就退回等分"""
    alpha = sheet.getchannel("A").point(lambda a: 255 if a > 24 else 0)
    filled = [alpha.crop((x, 0, x + 1, sheet.height)).getbbox() is not None for x in range(sheet.width)]
    runs, start = [], None
    for x, f in enumerate(filled + [False]):
        if f and start is None:
            start = x
        elif not f and start is not None:
            runs.append([start, x])
            start = None
    # 很窄的碎片(髮絲、光點)併到最近的那一段
    runs = [r for r in runs if r[1] - r[0] > 8]
    while len(runs) > frames:
        gaps = [runs[i + 1][0] - runs[i][1] for i in range(len(runs) - 1)]
        i = gaps.index(min(gaps))
        runs[i:i + 2] = [[runs[i][0], runs[i + 1][1]]]
    if len(runs) == frames:
        return [tuple(r) for r in runs]
    width = sheet.width // frames
    return [(i * width, (i + 1) * width) for i in range(frames)]


def split_walk(outfit: str, path: Path, frames: int = 4) -> None:
    sheet = Image.open(path).convert("RGBA")
    cells = []
    for left, right in columns(sheet, frames):
        cell = sheet.crop((left, 0, right, sheet.height))
        alpha = cell.getchannel("A").point(lambda a: 255 if a > 24 else 0)
        cells.append(cell.crop(alpha.getbbox()))
    # 以最高的那格為 475px,其餘同比例
    scale = HEIGHTS["walk"] / max(c.height for c in cells)
    for i, cell in enumerate(cells, start=1):
        size = (round(cell.width * scale), round(cell.height * scale))
        cell = cell.resize(size, Image.LANCZOS)
        # 桌寵把圖置中畫;各格跨出去的腳不同,直接置中頭會左右抖 →
        # 左右補透明邊,讓頭(上面 35% 的重心)落在正中間,只剩腳在動
        head = cell.getchannel("A").crop((0, 0, cell.width, int(cell.height * 0.35)))
        xs = [x for x in range(head.width) for y in range(0, head.height, 4) if head.getpixel((x, y)) > 24]
        center = sum(xs) / len(xs) if xs else cell.width / 2
        half = max(center, cell.width - center)
        canvas = Image.new("RGBA", (round(half * 2), cell.height))
        canvas.paste(cell, (round(half - center), 0))
        canvas.save(OUT / outfit / f"walk{i}.png", optimize=True)
        print(f"{outfit}/walk{i}.png  {canvas.width}×{canvas.height}")


if __name__ == "__main__":
    main()
