package golangJwtAuth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func (j *JWTAuth) Create(r *http.Request, w http.ResponseWriter, u *AuthData) (*TokenResult, error) {
	if u == nil {
		j.Logger.Create(true, "Auth data is required")
		return nil, fmt.Errorf("Auth data is required")
	}

	dateNow := time.Now()
	jwtID := uuid.New().String()
	fp := j.getFingerprint(r)

	refreshId, err := j.createRefreshId(u.ID, u.Name, u.Email, fp, jwtID)
	if err != nil {
		j.Logger.Create(true,
			"Failed to create Refresh ID",
			fmt.Sprintf("Auth ID: %s", u.ID),
			err.Error(),
		)
		return nil, fmt.Errorf("Failed to create Refresh ID: %v", err)
	}

	claims := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"id":         u.ID,
		"name":       u.Name,
		"email":      u.Email,
		"thumbnail":  u.Thumbnail,
		"scope":      u.Scope,
		"role":       u.Role,
		"level":      u.Level,
		"fp":         fp,
		"jti":        jwtID,
		"refresh_id": refreshId,
		"exp":        dateNow.Add(j.Config.AccessTokenExpires).Unix(),
		"iat":        dateNow.Unix(),
		"nbf":        dateNow.Unix(),
	})

	accessToken, err := claims.SignedString(j.Config.PrivateKeyPEM)
	if err != nil {
		j.Logger.Create(true,
			"Failed to sign access token",
			fmt.Sprintf("Auth ID: %s", u.ID),
			err.Error(),
		)
		return nil, fmt.Errorf("Failed to sign access token: %v", err)
	}

	j.setCookie(w, j.Config.AccessTokenCookieKey, accessToken, dateNow.Add(j.Config.AccessTokenExpires))
	j.setCookie(w, j.Config.RefreshIdCookieKey, refreshId, dateNow.Add(j.Config.RefreshIdExpires))

	refreshData := RefreshData{
		Data:        u,
		Version:     1,
		Fingerprint: fp,
		EXP:         dateNow.Add(j.Config.AccessTokenExpires).Unix(),
		IAT:         dateNow.Unix(),
		JTI:         jwtID,
	}
	refreshDataJson, err := json.Marshal(refreshData)
	if err != nil {
		j.Logger.Create(true,
			"Failed to parse refresh data",
			fmt.Sprintf("Auth ID: %s", u.ID),
			err.Error(),
		)
		return nil, fmt.Errorf("Failed to parse refresh data: %v", err)
	}

	pipe := j.Redis.TxPipeline()
	pipe.SetEx(j.Context, "refresh:"+refreshId, string(refreshDataJson), j.Config.RefreshIdExpires)
	pipe.SetEx(j.Context, "jti:"+jwtID, "1", j.Config.AccessTokenExpires)
	_, err = pipe.Exec(j.Context)
	if err != nil {
		j.Logger.Create(true,
			"Failed to store RefreshID/JTI in redis",
			fmt.Sprintf("Auth ID: %s", u.ID),
			err.Error(),
		)
		return nil, fmt.Errorf("Failed to store Refresh ID/JTI in redis: %v", err)
	}

	j.Logger.Create(false,
		"Created access token successfully",
		fmt.Sprintf("Auth ID: %s", u.ID),
	)

	return &TokenResult{
		Token:     accessToken,
		RefreshId: refreshId,
	}, nil
}
