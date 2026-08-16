package models

import "gorm.io/plugin/soft_delete"

func softDeleteAt(v int64) soft_delete.DeletedAt {
	if v <= 0 {
		return 0
	}
	return soft_delete.DeletedAt(v)
}
