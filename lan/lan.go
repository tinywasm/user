package lan

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

type Authenticator struct {
	db  *orm.DB
	ids model.IDGenerator
}

func New(db *orm.DB, ids model.IDGenerator) *Authenticator {
	return &Authenticator{db: db, ids: ids}
}

func (a *Authenticator) Name() string {
	return "lan"
}

func (a *Authenticator) Mount(r router.Router, module any) {}

func (a *Authenticator) ValidateRUT(rut string) (string, error) {
	rut = fmt.Convert(rut).TrimSpace().String()
	rut = fmt.Convert(rut).Replace(".", "").Replace("-", "").String()

	if len(rut) < 2 { // At least 1 digit + 1 DV
		return "", user.ErrInvalidRUT
	}

	bodyStr := rut[:len(rut)-1]
	dvStr := fmt.ToUpper(rut[len(rut)-1:])

	if _, err := fmt.Convert(bodyStr).Int(); err != nil {
		return "", user.ErrInvalidRUT
	}

	sum := 0
	multiplier := 2
	for i := len(bodyStr) - 1; i >= 0; i-- {
		digit := int(bodyStr[i] - '0')
		sum += digit * multiplier
		multiplier++
		if multiplier > 7 {
			multiplier = 2
		}
	}

	expectedDV := 11 - (sum % 11)
	var expectedDVStr string
	if expectedDV == 11 {
		expectedDVStr = "0"
	} else if expectedDV == 10 {
		expectedDVStr = "K"
	} else {
		expectedDVStr = fmt.Convert(expectedDV).String()
	}

	if dvStr != expectedDVStr {
		return "", user.ErrInvalidRUT
	}

	return bodyStr + "-" + expectedDVStr, nil
}

func (a *Authenticator) ExtractClientIP(ctx router.Context, trustProxy bool) string {
	if trustProxy {
		xff := ctx.GetHeader("X-Forwarded-For")
		if xff != "" {
			parts := fmt.Split(xff, ",")
			return fmt.Convert(parts[0]).TrimSpace().String()
		}
		xri := ctx.GetHeader("X-Real-IP")
		if xri != "" {
			return fmt.Convert(xri).TrimSpace().String()
		}
	}

	if addr, ok := ctx.Value("RemoteAddr").(string); ok {
		parts := fmt.Split(addr, ":")
		if len(parts) > 0 {
			return parts[0]
		}
		return addr
	}

	return ""
}
