package stimuli

import (
	"goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/img"
)

// Picture represents an image stimulus.
type Picture struct {
	FilePath string
	Memory   []byte
	Position sdl.FPoint
	Texture  *sdl.Texture
	Width    float32
	Height   float32
}

func NewPicture(filePath string, x, y float32) *Picture {
	return &Picture{
		FilePath: filePath,
		Position: sdl.FPoint{X: x, Y: y},
	}
}

// NewPictureFromMemory creates a new Picture stimulus from embedded data.
func NewPictureFromMemory(data []byte, x, y float32) *Picture {
	return &Picture{
		Memory:   data,
		Position: sdl.FPoint{X: x, Y: y},
	}
}

// preload prepares the texture from the file or memory.
func (p *Picture) preload(screen *io.Screen) error {
	var surface *sdl.Surface
	var err error

	if p.Memory != nil {
		ioStream, err := sdl.IOFromBytes(p.Memory)
		if err != nil {
			return err
		}
		surface, err = img.LoadIO(ioStream, true)
		if err != nil {
			return err
		}
	} else {
		surface, err = img.Load(p.FilePath)
		if err != nil {
			return err
		}
	}
	defer surface.Destroy()

	p.Width = float32(surface.W)
	p.Height = float32(surface.H)

	texture, err := screen.Renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return err
	}
	p.Texture = texture
	return nil
}

func (p *Picture) Preload() error {
	return nil
}

func (p *Picture) Draw(screen *io.Screen) error {
	if p.Texture == nil {
		if err := p.preload(screen); err != nil {
			return err
		}
	}
	
	destX, destY := screen.CenterToSDL(p.Position.X, p.Position.Y)
	// Centering the image at the target position
	destRect := &sdl.FRect{
		X: destX - p.Width/2,
		Y: destY - p.Height/2,
		W: p.Width,
		H: p.Height,
	}
	
	return screen.Renderer.RenderTexture(p.Texture, nil, destRect)
}

func (p *Picture) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := p.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (p *Picture) Unload() error {
	if p.Texture != nil {
		p.Texture.Destroy()
		p.Texture = nil
	}
	return nil
}

func (p *Picture) GetPosition() sdl.FPoint {
	return p.Position
}

func (p *Picture) SetPosition(pos sdl.FPoint) {
	p.Position = pos
}
