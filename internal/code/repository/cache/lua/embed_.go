package scripts

import _ "embed"

//go:embed setCode.lua
var SetCodeLua string

//go:embed verifyCode.lua
var VerifyCode string
