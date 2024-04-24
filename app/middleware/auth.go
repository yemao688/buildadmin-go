package middleware

import (
	"go-build-admin/conf"
)

type Auth struct {
	config *conf.Configuration
}

func NewAuth(config *conf.Configuration) *Auth {
	return &Auth{
		config: config,
	}
}

// func (m *Auth) Handler(guardName string) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		tokenStr := c.Request.Header.Get("Authorization")
// 		if tokenStr == "" {
// 			response.FailByErr(c, cErr.Unauthorized("missing Authorization header"))
// 			c.Abort()
// 			return
// 		}
// 		tokenStr = tokenStr[len(jwtS.TokenType)+1:]

// 		token, err := jwt.ParseWithClaims(tokenStr, &jwtS.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
// 			return []byte(m.config.Jwt.Secret), nil
// 		})
// 		if err != nil || m.jwtS.IsInBlacklist(c, tokenStr) {
// 			response.FailByErr(c, cErr.Unauthorized("登录授权已失效"))
// 			c.Abort()
// 			return
// 		}
// 		claims := token.Claims.(*jwtS.CustomClaims)
// 		if claims.Issuer != guardName && claims.Valid() != nil {
// 			response.FailByErr(c, cErr.Unauthorized("登录授权已失效"))
// 			c.Abort()
// 			return
// 		}
// 		c.Set("token", token)
// 		c.Set("id", claims.Id)
// 	}
// }
