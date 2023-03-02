package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type Source int

const (
	Mcbbs Source = iota
	BMCLAPI
	Mojang
)

func (source Source) String() string {
	switch source {
	case Mcbbs:
		return "mcbbs"
	case BMCLAPI:
		return "bmclapi"
	case Mojang:
		return "mojang"
	default:
		return "mojang"
	}
}

type User int

const (
	MojangLogin User = iota
	MicrosoftLogin
)

func (u User) String() string {
	switch u {
	case MojangLogin:
		return "mojang"
	case MicrosoftLogin:
		return "Microsoft"
	default:
		return "mojang"
	}
}

type McDownloader struct {
	SourceType    Source
	UserName      string
	UserLoginType string
	UserType      User
	McDir         string
	VersionJson   string
	versionJson   gjson.Result
	Cp            string
	PassWord      string
}
type NotArray struct{}

func (n *NotArray) Error() string {
	return "not array"
}
func NewNotArray() *NotArray {
	return &NotArray{}
}

type NotObj struct{}

func (n *NotObj) Error() string {
	return "not array"
}
func NewNotObj() *NotArray {
	return &NotArray{}
}

type VersionNotFound struct {
	errId string
}

func (v *VersionNotFound) Error() string {
	return "id \"" + v.errId + "\" is not found"
}
func NewVersionNotFound(errid string) *VersionNotFound {
	return &VersionNotFound{errId: errid}
}

type HashNotSame struct {
	Need string
	Got  string
}

func (h *HashNotSame) Error() string {
	return "hash not same\ngot: " + h.Got + "\nneed: " + h.Need
}
func NewHashNotSame(need, got string) *HashNotSame {
	return &HashNotSame{Need: need, Got: got}
}
func NewMcDownloader(sourceType, username, userloginType, userType, mcDir, password string, needVer string) (*McDownloader, error) {
	mcDir, err := filepath.Abs(mcDir)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(mcDir, 0666)
	if err != nil {
		return nil, err
	}
	versionDir := filepath.Join(mcDir, "versions")
	err = os.MkdirAll(versionDir, 0666)
	if err != nil {
		return nil, err
	}
	var versionInfoUrl string = "http://launchermeta.mojang.com/mc/game/version_manifest_v2.json"
	var rSourceType Source = Mojang
	switch sourceType {
	case "mcbbs":
		rSourceType = Mcbbs
		versionInfoUrl = "https://download.mcbbs.net/mc/game/version_manifest_v2.json"
	case "bmclapi":
		rSourceType = BMCLAPI
		versionInfoUrl = "https://bmclapi2.bangbang93.com/mc/game/version_manifest_v2.json"
	}
	verInfoByte, err := GetByteInInternet(versionInfoUrl)
	if err != nil {
		return nil, err
	}
	err = WriteFmtJsonBytes("version_manifest_v2.json", verInfoByte)
	if err != nil {
		return nil, err
	}
	verInfoString := string(verInfoByte)
	verInfo := gjson.Get(verInfoString, "versions")
	if !verInfo.IsArray() {
		return nil, NewNotArray()
	}
	hasVer := false
	var verInfos gjson.Result
	for _, v := range verInfo.Array() {
		if needVer == v.Get("id").String() {
			hasVer = true
			verInfos = v
		}
	}
	if !hasVer {
		return nil, NewVersionNotFound(needVer)
	}
	verPath := filepath.Join(versionDir, needVer)
	err = os.MkdirAll(verPath, 0666)
	if err != nil {
		return nil, err
	}
	verJsonPath := filepath.Join(verPath, needVer+".json")
	versionUrl := verInfos.Get("url").String()
	versionSha1 := verInfos.Get("sha1").String()
	switch sourceType {
	case "mcbbs":
		versionUrl = ReplaceByMap(versionUrl, map[string]string{
			"https://piston-meta.mojang.com":  "https://download.mcbbs.net",
			"https://launchermeta.mojang.com": "https://download.mcbbs.net",
			"https://launcher.mojang.com":     "https://download.mcbbs.net",
		})
	case "bmclapi":
		versionUrl = ReplaceByMap(versionUrl, map[string]string{
			"https://piston-meta.mojang.com":  "https://bmclapi2.bangbang93.com",
			"https://launchermeta.mojang.com": "https://bmclapi2.bangbang93.com",
			"https://launcher.mojang.com":     "https://bmclapi2.bangbang93.com",
		})
	}
	versionString, err := GetStrFmtJsonInInternetWithHash(versionUrl, "sha1", versionSha1)
	if err != nil {
		return nil, err
	}
	err = DownloadFmtJsonWithHash(verJsonPath, versionUrl, "sha1", versionSha1)
	if err != nil {
		return nil, err
	}
	var ruserloginType User = MojangLogin
	if userloginType == "Microsoft" {
		ruserloginType = MicrosoftLogin
	}
	return &McDownloader{UserType: ruserloginType, SourceType: rSourceType, UserName: username, UserLoginType: userloginType, VersionJson: versionString, versionJson: gjson.Parse(versionString), McDir: mcDir, PassWord: password}, nil
}
func (self *McDownloader) GetLib() error {
	if !self.versionJson.Get("libraries").IsArray() {
		return NewNotArray()
	}
	var cp string
	libdir := filepath.Join(self.McDir, "libraries")
	err := os.MkdirAll(libdir, 0666)
	if err != nil {
		return err
	}
	lib := self.versionJson.Get("libraries").Array()
	for _, v := range lib {
		rules := v.Get("rules")
		if rules.Exists() {
			var notsame bool = false
			for _, vr := range rules.Array() {
				action := vr.Get("action")
				if action.Exists() {
					if action.String() != "allow" {
						continue
					}
				}
				oses := vr.Get("os")
				if oses.Exists() {
					oses.ForEach(func(key, value gjson.Result) bool {
						if key.String() == "name" && value.String() != "windows" {
							notsame = true
							return false
						}
						if key.String() == "arch" && value.String() != "x"+strings.TrimLeft(runtime.GOARCH, "amd") {
							notsame = true
							return false
						}
						return true
					})
				}
			}
			if notsame {
				continue
			}
		}
		downloads := v.Get("downloads")
		if !downloads.Exists() {
			continue
		}
		artifact := downloads.Get("artifact")
		if !artifact.Exists() {
			continue
		}
		pathResult := artifact.Get("path")
		if !pathResult.Exists() {
			continue
		}
		paths := pathResult.String()
		paths = ReplaceByMap(paths, map[string]string{
			"/": "\\",
		})
		libpath := filepath.Join(libdir, paths)
		libdirs, _ := filepath.Split(libpath)
		err = os.MkdirAll(libdirs, 0666)
		if err != nil {
			return err
		}
		liburlResult := artifact.Get("url")
		if !liburlResult.Exists() {
			continue
		}
		liburl := liburlResult.String()
		switch self.SourceType {
		case Mcbbs:
			liburl = ReplaceByMap(liburl, map[string]string{
				"https://libraries.minecraft.net": "https://download.mcbbs.net/maven",
			})
		case BMCLAPI:
			liburl = ReplaceByMap(liburl, map[string]string{
				"https://libraries.minecraft.net": "https://bmclapi2.bangbang93.com/maven",
			})
		}
		libsha1Result := artifact.Get("sha1")
		if !libsha1Result.Exists() {
			continue
		}
		libsha1 := libsha1Result.String()
		err = DownloadWithHash(libpath, liburl, "sha1", libsha1)
		if err != nil {
			return err
		}
		cp += libpath + ";"
		println(libpath)
		time.Sleep(800)
	}
	self.Cp = cp
	return nil
}
func (self *McDownloader) Launch(startname string, isdomo bool) (string, error) {
	switch self.UserLoginType {
	case "littleskin":
		tokens := RandStringBytes(len("ssssdddadfsdfsfsffsxxdsfewfsdf"))
		value := map[string]interface{}{
			"agent": map[string]interface{}{
				"name":    "Minecraft",
				"version": self.versionJson.Get("complianceLevel").Int(),
			},
			"username":    self.UserName,
			"password":    self.PassWord,
			"clientToken": tokens,
			"requestUser": false,
		}
		gets, err := PostMapGotBytes("https://littleskin.cn/api/yggdrasil/authserver/authenticate", map[string]string{
			"Content-Type": "application/json",
		}, value)
		if err != nil {
			return "", err
		}
		getstr := string(gets)
		AssertsTrue(gjson.Get(getstr, "clientToken").String() == tokens)
		availableProfiles := gjson.Get(getstr, "availableProfiles")
		if !availableProfiles.Exists() || !availableProfiles.IsArray() {
			return "", NewNotArray()
		}
		var uuid string = ""
		var hasName bool = false
		for _, v := range availableProfiles.Array() {
			if v.Get("name").String() == startname {
				hasName = true
				uuid = v.Get("id").String()
			}
		}
		if !hasName {
			return "", errors.New("no name you have")
		}
		accessToken := gjson.Get(getstr, "accessToken").String()
		assetsDir := filepath.Join(self.McDir, "assets")
		indexDir := filepath.Join(assetsDir, "indexes")
		err = os.MkdirAll(assetsDir, 0666)
		if err != nil {
			return "", err
		}
		err = os.MkdirAll(indexDir, 0666)
		if err != nil {
			return "", err
		}
		assetsIndexResult := self.versionJson.Get("assetIndex")
		if !assetsIndexResult.Exists() {
			return "", err
		}
		assetsIndexName := assetsIndexResult.Get("id").String()
		indexJsonPath := filepath.Join(indexDir, assetsIndexName+".json")
		indexJson, err := GetStrFmtJsonInInternetWithHash(assetsIndexResult.Get("url").String(), "sha1", assetsIndexResult.Get("sha1").String())
		if err != nil {
			return "", err
		}
		err = WriteBytes(indexJsonPath, []byte(indexJson))
		if err != nil {
			return "", err
		}
		self.Cp = "\"" + self.Cp + "\""
		fmt.Printf("isdomo:%t\n", isdomo)
		vername := self.versionJson.Get("id").String()
		versiondir := filepath.Join(self.McDir, "versions", vername)
		nativedir := filepath.Join(versiondir, "natives")
		err = os.MkdirAll(nativedir, 0666)
		if err != nil {
			return "", err
		}
		version_type := self.versionJson.Get("type").String()
		mainclassname := self.versionJson.Get("mainClass").String()
		args := self.versionJson.Get("arguments")
		gameargs := []string{}
		gamearg := args.Get("game")
		jvmargs := []string{}
		jvmarg := args.Get("jvm")
		err = self.GetObj(indexJson)
		if err != nil {
			return "", err
		}
		for _, v := range gamearg.Array() {
			if v.Type.String() == "String" {
				gameargs = append(gameargs, v.String())
				continue
			}
			rules := v.Get("rules")
			var notsame bool = IsRuleSameFrom_gjson_Result(rules, isdomo)
			if notsame {
				continue
			}
			value := v.Get("value")
			if value.Type.String() == "String" {
				gameargs = append(gameargs, value.String())
				continue
			}
			if !value.IsArray() {
				return "", errors.New("not array")
			}
			for _, vs := range value.Array() {
				gameargs = append(gameargs, vs.String())
			}
		}
		gamestrargs := ""
		for _, v := range gameargs {
			gamestrargs += v + " "
		}
		gamestrargs = strings.TrimRight(gamestrargs, " ")
		gamestrargs = ReplaceByMap(gamestrargs, map[string]string{
			"${auth_player_name}":  startname,
			"${version_name}":      vername,
			"${game_directory}":    "\"" + self.McDir + "\"",
			"${assets_root}":       "\"" + assetsDir + "\"",
			"${assets_index_name}": assetsIndexName,
			"${auth_uuid}":         uuid,
			"${auth_access_token}": accessToken,
			"${user_type}":         self.UserType.String(),
			"${version_type}":      version_type,
			"${resolution_width}":  "854",
			"${resolution_height}": "480",
		})
		clientid := "112321"
		if clientid != "" {
			gamestrargs = strings.ReplaceAll(gamestrargs, "${clientid}", clientid)
		}
		auth_xuid := "114514"
		if auth_xuid != "" {
			gamestrargs = strings.ReplaceAll(gamestrargs, "${auth_xuid}", auth_xuid)
		}
		for _, v := range jvmarg.Array() {
			if v.Type.String() == "String" {
				jvmargs = append(jvmargs, v.String())
				continue
			}
			rules := v.Get("rules")
			var notsame bool = IsRuleSameFrom_gjson_Result(rules, isdomo)
			if notsame {
				continue
			}
			value := v.Get("value")
			if value.Type.String() == "String" {
				jvmargs = append(jvmargs, value.String())
				continue
			}
			if !value.IsArray() {
				return "", errors.New("not array")
			}
			for _, vs := range value.Array() {
				jvmargs = append(jvmargs, vs.String())
			}
		}
		jvmStrargs := ""
		for _, v := range jvmargs {
			jvmStrargs += v + " "
		}
		jvmStrargs = ReplaceByMap(jvmStrargs, map[string]string{
			"${natives_directory}": "\"" + nativedir + "\"",
			"${launcher_name}":     "newL",
			"${launcher_version}":  "27",
			"${classpath}":         self.Cp,
			"-Dos.name=Windows 10": "-Dos.name=\"Windows 10\"",
		})
		allarg := jvmStrargs + " " + mainclassname + " " + gamestrargs
		javaVersion := fmt.Sprintf("%d", self.versionJson.Get("javaVersion").Get("majorVersion").Int())
		configs, err := ReadString("config.json")
		if err != nil {
			return "", err
		}
		configjson := gjson.Parse(configs)
		hasJavaVersion := configjson.Get("javaversions")
		need := hasJavaVersion.Get(javaVersion)
		if !need.Exists() {
			println("Has not java version needs.")
			return "", nil
		}
		needPath := "\"" + need.String() + "\""
		command := "@echo off\n" + needPath + " " + allarg
		println(command)
		return command, nil
	}
	return "", nil
}
func (self *McDownloader) GetObj(assetIndex string) error {
	assetsDir := filepath.Join(self.McDir, "assets")
	needBackup := false
	if gjson.Get(assetIndex, "map_to_resources").Exists() {
		if gjson.Get(assetIndex, "map_to_resources").IsBool() {
			if gjson.Get(assetIndex, "map_to_resources").Bool() {
				needBackup = true
			}
		}
	}
	objdir := filepath.Join(assetsDir, "objects")
	err := os.RemoveAll(objdir)
	if err != nil {
		return err
	}
	objs := gjson.Get(assetIndex, "objects")
	if !objs.IsObject() {
		return NewNotObj()
	}
	var theErr error = nil
	objs.ForEach(func(key, value gjson.Result) bool {
		var rootUrl string = "https://resources.download.minecraft.net/"
		switch self.SourceType {
		case Mcbbs:
			rootUrl = "https://download.mcbbs.net/assets/"
		case BMCLAPI:
			rootUrl = "https://bmclapi2.bangbang93.com/assets/"
		}
		hashcode := value.Get("hash").String()
		twoHash := hashcode[:2]
		url := rootUrl + twoHash + "/" + hashcode

		path := filepath.Join(objdir, twoHash, hashcode)
		pathdir := filepath.Dir(path)
		err = os.MkdirAll(pathdir, 0666)
		if err != nil {
			theErr = err
			return false
		}
		objb, err := GetByteInInternet(url)
		if err != nil {
			theErr = err
			return false
		}
		err = WriteBytes(path, objb)
		if err != nil {
			theErr = err
			return false
		}
		if needBackup {
			backuppath := filepath.Join(assetsDir, "virtual", "legacy", ReplaceByMap(key.String(), map[string]string{
				"/": "\\",
			}))
			err = os.MkdirAll(filepath.Dir(backuppath), 0666)
			if err != nil {
				theErr = err
				return false
			}
			err = WriteBytes(backuppath, objb)
			if err != nil {
				theErr = err
				return false
			}
		}
		println(hashcode)
		time.Sleep(800)
		return true
	})
	if theErr != nil {
		return theErr
	}
	return nil
}
func (self *McDownloader) GetClient() error {
	println("downloading: client")
	client := self.versionJson.Get("downloads").Get("client")
	url := client.Get("url").String()
	clientSha1 := client.Get("sha1").String()
	verid := self.versionJson.Get("id").String()
	verjar := filepath.Join(self.McDir, "versions", verid, verid+".jar")
	switch self.SourceType {
	case Mcbbs:
		url = ReplaceByMap(url, map[string]string{
			"https://piston-data.mojang.com":  "https://download.mcbbs.net",
			"https://launchermeta.mojang.com": "https://download.mcbbs.net",
			"https://launcher.mojang.com":     "https://download.mcbbs.net",
		})
	case BMCLAPI:
		url = ReplaceByMap(url, map[string]string{
			"https://piston-data.mojang.com":  "https://bmclapi2.bangbang93.com",
			"https://launchermeta.mojang.com": "https://bmclapi2.bangbang93.com",
			"https://launcher.mojang.com":     "https://bmclapi2.bangbang93.com",
		})
	}
	err := DownloadWithHash(verjar, url, "sha1", clientSha1)
	if err != nil {
		return err
	}
	self.Cp += verjar
	println("downloaded: client")
	return nil
}

type McBackup struct {
	Cp          string
	VersionJson string
	McDir       string
}

func NewMcBackup(cp, verjson, mcdir string) *McBackup {
	return &McBackup{Cp: cp, VersionJson: verjson, McDir: mcdir}
}
