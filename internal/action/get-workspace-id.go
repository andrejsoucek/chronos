package action

import (
	"github.com/andrejsoucek/chronos/pkg/clockify"
)

func GetWorkspaceID(clockify *clockify.Clockify) (string, error) {
	return clockify.GetWorkspaceID()
}
