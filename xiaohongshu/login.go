package xiaohongshu

import (
        "context"
        "strings"
        "time"

        "github.com/go-rod/rod"
        "github.com/pkg/errors"
)

type LoginAction struct {
        page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
        return &LoginAction{page: page}
}

const exploreURL = "https://www.xiaohongshu.com/explore"

// ensureExplore 会在需要时跳转到小红书探索页。
// forceReload 为 true 时会强制刷新页面以获取新的登录态或二维码。
func (a *LoginAction) ensureExplore(ctx context.Context, forceReload bool) {
        page := a.page.Context(ctx)

        shouldNavigate := forceReload
        if !shouldNavigate {
                info := page.MustInfo()
                shouldNavigate = !strings.HasPrefix(info.URL, exploreURL)
        }

        if shouldNavigate {
                page.MustNavigate(exploreURL).MustWaitLoad()
                time.Sleep(2 * time.Second)
        }
}

// isLoggedInOnCurrentPage 判断当前页面是否已登录。
func (a *LoginAction) isLoggedInOnCurrentPage(ctx context.Context) (bool, error) {
        page := a.page.Context(ctx)

        res, err := page.Eval(`() => {
                const el = document.querySelector('.main-container .user .link-wrapper .channel');
                return Boolean(el);
        }`)
        if err != nil {
                return false, errors.Wrap(err, "check login status failed")
        }

        return res.Value.Bool(), nil
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, error) {
        a.ensureExplore(ctx, false)
        return a.isLoggedInOnCurrentPage(ctx)
}

func (a *LoginAction) Login(ctx context.Context) error {
        a.ensureExplore(ctx, true)
        _, err := a.WaitForLogin(ctx, 2*time.Minute)
        return err
}

// FetchLoginQRCode 获取登录二维码的 base64 图片数据。
func (a *LoginAction) FetchLoginQRCode(ctx context.Context) (string, error) {
        a.ensureExplore(ctx, true)

        page := a.page.Context(ctx)

        script := `() => {
                const selectors = [
                        '.qrcode-img-box img',
                        '.login-dialog img',
                        '.login-modal img',
                        '.login-container img'
                ];
                for (const sel of selectors) {
                        const el = document.querySelector(sel);
                        if (el && typeof el.src === 'string' && el.src.startsWith('data:image/png;base64')) {
                                return el.src;
                        }
                }
                const imgs = Array.from(document.querySelectorAll('img'));
                for (const img of imgs) {
                        if (img && img.src && img.src.startsWith('data:image/png;base64') && img.src.length > 1024) {
                                return img.src;
                        }
                }
                return '';
        }`

        deadline := time.Now().Add(30 * time.Second)
        for time.Now().Before(deadline) {
                res, err := page.Eval(script)
                if err != nil {
                        return "", errors.Wrap(err, "evaluate login qrcode script")
                }

                if src := res.Value.Str(); src != "" {
                        return src, nil
                }

                time.Sleep(1 * time.Second)
        }

        return "", errors.New("login qrcode not found")
}

// WaitForLogin 在当前页面等待登录成功，返回是否成功。
func (a *LoginAction) WaitForLogin(ctx context.Context, timeout time.Duration) (bool, error) {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()

        timeoutTimer := time.NewTimer(timeout)
        defer timeoutTimer.Stop()

        for {
                select {
                case <-ctx.Done():
                        return false, ctx.Err()
                case <-timeoutTimer.C:
                        return false, nil
                case <-ticker.C:
                        loggedIn, err := a.isLoggedInOnCurrentPage(ctx)
                        if err != nil {
                                return false, err
                        }
                        if loggedIn {
                                return true, nil
                        }
                }
        }
}
