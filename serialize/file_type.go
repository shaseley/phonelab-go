package serialize

import "github.com/gurupras/go-easyfiles"

type FileType easyfiles.FileType

const (
	GZ_TRUE  FileType = FileType(easyfiles.GZ_TRUE)
	GZ_FALSE FileType = FileType(easyfiles.GZ_FALSE)
)
