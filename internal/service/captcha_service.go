// SPDX-License-Identifier: GPL-3.0-or-later

package service

import (
	"image/color"
	"time"

	"github.com/mojocn/base64Captcha"
)

// CaptchaService 验证码服务
// 使用独立的内存存储（限定容量与过期时间），避免全局 DefaultMemStore 被并发请求挤占。
type CaptchaService struct {
	store base64Captcha.Store
}

func NewCaptchaService(capacity int) *CaptchaService {
	if capacity <= 0 {
		capacity = 1000
	}
	// 验证码 5 分钟过期；超出容量时回收最旧的条目。
	store := base64Captcha.NewMemoryStore(capacity, 5*time.Minute)
	return &CaptchaService{store: store}
}

func (s *CaptchaService) Generate() (id string, b64 string, err error) {
	driver := &base64Captcha.DriverMath{
		Height:          40,
		Width:           120,
		NoiseCount:      0,
		ShowLineOptions: base64Captcha.OptionShowHollowLine,
		BgColor:         &color.RGBA{R: 255, G: 255, B: 255, A: 255},
		Fonts:           []string{"wqy-microhei.ttc"},
	}
	c := base64Captcha.NewCaptcha(driver.ConvertFonts(), s.store)
	captchaID, captchaB64, _, err := c.Generate()
	if err != nil {
		return "", "", err
	}
	return captchaID, captchaB64, nil
}

func (s *CaptchaService) Validate(id, answer string) bool {
	return s.store.Verify(id, answer, true)
}
