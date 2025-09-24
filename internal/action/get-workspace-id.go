package action

import (
	"github.com/andrejsoucek/chronos/pkg"
)

func GetWorkspaceID(clockify *pkg.Clockify) (string, error) {
	return clockify.GetWorkspaceID()
}
