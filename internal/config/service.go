package config

import (
    "os"
    "strings"
)

type Service struct {
    Name      string
    PprofAddr string
}

func LoadService(name string, defaultPprofAddr string) Service {
    upperName := strings.ToUpper(name)

    if serviceAddr := os.Getenv("METIN2_" + upperName + "_PPROF_ADDR"); serviceAddr != "" {
        return Service{Name: name, PprofAddr: serviceAddr}
    }

    if globalAddr := os.Getenv("METIN2_PPROF_ADDR"); globalAddr != "" {
        return Service{Name: name, PprofAddr: globalAddr}
    }

    return Service{Name: name, PprofAddr: defaultPprofAddr}
}
