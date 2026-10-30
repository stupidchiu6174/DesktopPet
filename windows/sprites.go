package main

import (
	"embed"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	xdraw "golang.org/x/image/draw"
)

// 素材:build.sh 把 Sources/DesktopPet/Resources/Pet 複製到這裡再編進 exe
//
//go:embed pet
var embedded embed.FS

type outfit int

const (
	witch outfit = iota
	qipao
	kimono
)

var outfits = []outfit{witch, qipao, kimono}

func (o outfit) name() string { return [...]string{"witch", "qipao", "kimono"}[o] }

func (o outfit) title() string { return [...]string{"魔女裝", "旗袍", "和服"}[o] }

func (o outfit) greeting() string {
	return [...]string{"魔女裝最自在了～", "換上旗袍了！好看嗎？", "和服…走路要小步一點"}[o]
}

func outfitNamed(s string) (outfit, bool) {
	for _, o := range outfits {
		if o.name() == s {
			return o, true
		}
	}
	return witch, false
}

type pose string

const (
	stand     pose = "stand"
	blink     pose = "blink"
	walk      pose = "walk"
	sit       pose = "sit"
	sleep     pose = "sleep"
	happy     pose = "happy"
	surprised pose = "surprised"
	held      pose = "held"
	dance     pose = "dance"
)

// 素材像素 → 點(素材以 3 倍解析度生成,站姿約 160pt 高)
const spriteScale = 1.0 / 3.0

// 一張已經縮到螢幕像素大小的圖;w、h 是點數
type sprite struct {
	img  *image.RGBA
	w, h float64
}

// 素材快取。缺的姿勢退回站姿。
type sprites struct {
	// 每一點幾個螢幕像素
	pixelScale float64
	cache      map[string]*sprite
	frames     map[outfit]int
}

func newSprites(pixelScale float64) *sprites {
	return &sprites{pixelScale: pixelScale, cache: map[string]*sprite{}, frames: map[outfit]int{}}
}

func (s *sprites) image(o outfit, p pose, frame int) *sprite {
	// 走路有循環格(walk1…walkN)就輪播,沒有就用單張 walk
	if p == walk {
		if n := s.walkFrames(o); n > 0 {
			if img := s.load(o, "walk"+strconv.Itoa(frame%n+1)); img != nil {
				return img
			}
		}
	}
	for _, try := range []struct {
		o outfit
		p pose
	}{{o, p}, {o, stand}, {witch, p}, {witch, stand}} {
		if img := s.load(try.o, string(try.p)); img != nil {
			return img
		}
	}
	return nil
}

// 這套衣服有幾格走路循環
func (s *sprites) walkFrames(o outfit) int {
	if n, ok := s.frames[o]; ok {
		return n
	}
	n := 0
	for s.load(o, "walk"+strconv.Itoa(n+1)) != nil {
		n++
	}
	s.frames[o] = n
	return n
}

// 開發時可以把新素材放在 %APPDATA%\DesktopPet\Pet\,優先讀這裡
func userRoot() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "DesktopPet", "Pet")
}

func (s *sprites) load(o outfit, name string) *sprite {
	key := o.name() + "/" + name
	if cached, ok := s.cache[key]; ok {
		return cached
	}
	var src image.Image
	if root := userRoot(); root != "" {
		if f, err := os.Open(filepath.Join(root, o.name(), name+".png")); err == nil {
			src, _ = png.Decode(f)
			f.Close()
		}
	}
	if src == nil {
		if f, err := fs.Sub(embedded, "pet"); err == nil {
			if r, err := f.Open(key + ".png"); err == nil {
				src, _ = png.Decode(r)
				r.Close()
			}
		}
	}
	if src == nil {
		s.cache[key] = nil
		return nil
	}
	b := src.Bounds()
	w, h := float64(b.Dx())*spriteScale, float64(b.Dy())*spriteScale
	// 先用好的濾鏡縮到螢幕像素大小,之後每格的搖擺、壓扁只是接近 1:1 的雙線性取樣
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(w*s.pixelScale+0.5)), max(1, int(h*s.pixelScale+0.5))))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	sp := &sprite{img: dst, w: w, h: h}
	s.cache[key] = sp
	return sp
}
