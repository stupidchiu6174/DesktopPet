package main

import (
	"image"
	"math"
)

// 桌寵視窗的大小(點):角色站在底部中間,對話泡泡在頭上
const winW, winH = 260.0, 260.0

// 一格畫面要畫的東西(對應 macOS 版 PetModel / PetView)
type frameState struct {
	sprite     *sprite
	facingLeft bool
	// 往上跳的高度(點)、左右搖擺角度(度)、上下壓扁(1 = 原狀)
	bob, tilt, squash float64
	// 換衣服的小特效,0…1
	poof float64
	// 泡泡文字的大小(點);沒有泡泡時是 0
	textW, textH float64
}

type canvas struct {
	k    float64 // 每一點幾個像素
	img  *image.RGBA
	hit  []uint8 // 角色本身的 alpha(不含影子、泡泡),拿來判斷點到的是不是她
	tmp  []float64
	tmp2 []float64
}

func newCanvas(k float64) *canvas {
	w, h := int(math.Round(winW*k)), int(math.Round(winH*k))
	return &canvas{k: k, img: image.NewRGBA(image.Rect(0, 0, w, h)), hit: make([]uint8, w*h),
		tmp: make([]float64, w*h), tmp2: make([]float64, w*h)}
}

func (c *canvas) size() (int, int) { return c.img.Rect.Dx(), c.img.Rect.Dy() }

// 畫一格;回傳泡泡文字該畫在哪(像素),沒有泡泡時 ok = false
func (c *canvas) draw(f frameState) (text image.Rectangle, ok bool) {
	clear(c.img.Pix)
	clear(c.hit)
	if f.sprite != nil {
		c.drawSprite(f)
	}
	if f.textW > 0 && f.sprite != nil {
		return c.drawBubble(f), true
	}
	return image.Rectangle{}, false
}

func (c *canvas) drawSprite(f frameState) {
	W, H := c.size()
	sp := f.sprite
	k := c.k
	scale := 1 - 0.4*f.poof
	opacity := 1 - 0.7*f.poof
	sx, sy := 2-f.squash, f.squash
	th := f.tilt * math.Pi / 180
	cos, sin := math.Cos(th), math.Sin(th)

	// 正向:角色座標(原點在腳底中央,y 朝下)→ 視窗像素
	forward := func(u, v float64) (float64, float64) {
		if f.facingLeft {
			u = -u
		}
		u, v = u*sx, v*sy
		x, y := u*cos-v*sin, u*sin+v*cos
		y -= f.bob
		x, y = x*scale, y*scale
		return (x + winW/2) * k, (y + winH) * k
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range [][2]float64{{-sp.w / 2, 0}, {sp.w / 2, 0}, {-sp.w / 2, -sp.h}, {sp.w / 2, -sp.h}} {
		x, y := forward(p[0], p[1])
		minX, minY, maxX, maxY = math.Min(minX, x), math.Min(minY, y), math.Max(maxX, x), math.Max(maxY, y)
	}
	x0, y0 := max(0, int(minX)-2), max(0, int(minY)-2)
	x1, y1 := min(W, int(maxX)+3), min(H, int(maxY)+3)

	src := sp.img
	iw, ih := float64(src.Rect.Dx()), float64(src.Rect.Dy())
	// 影子:黑色 18%,模糊 3 點,往下 2 點(先只記 alpha,最後墊在角色下面)
	shadow := c.tmp
	clear(shadow)
	for Y := y0; Y < y1; Y++ {
		for X := x0; X < x1; X++ {
			// 反向:視窗像素 → 角色座標 → 素材像素
			x := (float64(X)+0.5)/k - winW/2
			y := (float64(Y)+0.5)/k - winH
			x, y = x/scale, y/scale
			y += f.bob
			u, v := x*cos+y*sin, -x*sin+y*cos
			u, v = u/sx, v/sy
			if f.facingLeft {
				u = -u
			}
			r, g, b, a := bilinear(src, (u/sp.w+0.5)*iw-0.5, (1+v/sp.h)*ih-0.5)
			if a == 0 {
				continue
			}
			i := Y*c.img.Stride + X*4
			c.img.Pix[i] = uint8(r*opacity + 0.5)
			c.img.Pix[i+1] = uint8(g*opacity + 0.5)
			c.img.Pix[i+2] = uint8(b*opacity + 0.5)
			c.img.Pix[i+3] = uint8(a*opacity + 0.5)
			c.hit[Y*W+X] = uint8(a + 0.5)
			shadow[Y*W+X] = a / 255 * opacity
		}
	}
	radius := int(math.Round(3 * k / 2)) // 兩次方框模糊 ≈ 高斯
	off := int(math.Round(2 * k))
	bx0, by0 := max(0, x0-2*radius), max(0, y0-2*radius)
	bx1, by1 := min(W, x1+2*radius), min(H, y1+2*radius+off)
	for range 2 {
		boxBlur(shadow, c.tmp2, W, bx0, by0, bx1, by1, radius)
	}
	for Y := by0; Y < by1; Y++ {
		sY := Y - off
		if sY < 0 {
			continue
		}
		for X := bx0; X < bx1; X++ {
			sa := shadow[sY*W+X] * 0.18 * 255
			if sa < 0.5 {
				continue
			}
			i := Y*c.img.Stride + X*4
			// 影子墊在下面:dst = dst + shadow × (1 − dst.a)
			rest := 1 - float64(c.img.Pix[i+3])/255
			c.img.Pix[i+3] += uint8(sa*rest + 0.5)
		}
	}
}

// 預乘 alpha 的雙線性取樣,範圍外當透明
func bilinear(img *image.RGBA, x, y float64) (r, g, b, a float64) {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if x0 < -1 || y0 < -1 || x0 >= w || y0 >= h {
		return
	}
	for _, s := range [4]struct {
		dx, dy int
		wt     float64
	}{{0, 0, (1 - fx) * (1 - fy)}, {1, 0, fx * (1 - fy)}, {0, 1, (1 - fx) * fy}, {1, 1, fx * fy}} {
		px, py := x0+s.dx, y0+s.dy
		if px < 0 || py < 0 || px >= w || py >= h || s.wt == 0 {
			continue
		}
		i := py*img.Stride + px*4
		r += float64(img.Pix[i]) * s.wt
		g += float64(img.Pix[i+1]) * s.wt
		b += float64(img.Pix[i+2]) * s.wt
		a += float64(img.Pix[i+3]) * s.wt
	}
	return
}

// 在 [x0,x1)×[y0,y1) 裡做一次水平+垂直的方框模糊
func boxBlur(v, tmp []float64, stride, x0, y0, x1, y1, r int) {
	if r < 1 {
		return
	}
	n := float64(2*r + 1)
	for y := y0; y < y1; y++ {
		sum := 0.0
		for x := x0 - r; x <= x0+r; x++ {
			if x >= x0 && x < x1 {
				sum += v[y*stride+x]
			}
		}
		for x := x0; x < x1; x++ {
			tmp[y*stride+x] = sum / n
			if out := x - r; out >= x0 {
				sum -= v[y*stride+out]
			}
			if in := x + r + 1; in < x1 {
				sum += v[y*stride+in]
			}
		}
	}
	for x := x0; x < x1; x++ {
		sum := 0.0
		for y := y0 - r; y <= y0+r; y++ {
			if y >= y0 && y < y1 {
				sum += tmp[y*stride+x]
			}
		}
		for y := y0; y < y1; y++ {
			v[y*stride+x] = sum / n
			if out := y - r; out >= y0 {
				sum -= tmp[out*stride+x]
			}
			if in := y + r + 1; in < y1 {
				sum += tmp[in*stride+x]
			}
		}
	}
}

// 白色圓角泡泡 + 往下的小三角,在角色頭上 44 點
func (c *canvas) drawBubble(f frameState) image.Rectangle {
	k := c.k
	bw, bh := f.textW+20, f.textH+12
	left := winW/2 - bw/2
	top := winH - f.sprite.h - 44 - f.bob
	if top < 2 {
		top = 2
	}
	// 圓角矩形到邊的有號距離(點)
	box := func(x, y float64) float64 {
		const r = 12.0
		cx, cy := left+bw/2, top+bh/2
		qx := math.Abs(x-cx) - (bw/2 - r)
		qy := math.Abs(y-cy) - (bh/2 - r)
		out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
		return out + math.Min(math.Max(qx, qy), 0) - r
	}
	// 三角形(上寬 12、高 7,上緣比泡泡底高 1 點)
	tx, ty := winW/2, top+bh-1
	tri := func(x, y float64) float64 {
		d := ty - y // 上緣
		// 兩條斜邊
		nx, ny := 7.0, 6.0
		l := math.Hypot(nx, ny)
		d = math.Max(d, ((x-tx)*nx+(y-(ty+7))*ny)/l)
		d = math.Max(d, ((tx-x)*nx+(y-(ty+7))*ny)/l)
		return d
	}
	W, H := c.size()
	x0, y0 := max(0, int((left-6)*k)), max(0, int((top-6)*k))
	x1, y1 := min(W, int((left+bw+6)*k)+1), min(H, int((ty+14)*k)+1)
	for Y := y0; Y < y1; Y++ {
		for X := x0; X < x1; X++ {
			x, y := (float64(X)+0.5)/k, (float64(Y)+0.5)/k
			d := math.Min(box(x, y), tri(x, y)) * k // 像素
			white := math.Min(math.Max(0.5-d, 0), 1)
			// 柔和的影子:往下 1 點、模糊 3 點、黑色 20%
			sd := math.Min(box(x, y-1), tri(x, y-1))
			shade := 0.2 * math.Min(math.Max((1.5-sd)/4.5, 0), 1)
			a := white + shade*(1-white)
			if a <= 0 {
				continue
			}
			i := Y*c.img.Stride + X*4
			// 泡泡蓋在角色上面(預乘:白色 = a×255 的灰階)
			rest := 1 - a
			for ch := 0; ch < 3; ch++ {
				c.img.Pix[i+ch] = uint8(white*255 + float64(c.img.Pix[i+ch])*rest + 0.5)
			}
			c.img.Pix[i+3] = uint8(a*255 + float64(c.img.Pix[i+3])*rest + 0.5)
		}
	}
	return image.Rect(int((left+10)*k), int((top+6)*k), int((left+bw-10)*k+0.999), int((top+bh-6)*k+0.999))
}
