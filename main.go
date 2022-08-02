package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"time"

	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var args struct {
	ListAllKeys bool
	Key         string
	Delete      string
	Rename      bool
	History     string
	Copy        bool
	DeepCopy    bool
	GC          bool
}

func init() {
	flag.StringVar(&args.Key, "e", "", "edit key")
	flag.BoolVar(&args.ListAllKeys, "a", false, "list all keys")
	flag.StringVar(&args.Delete, "d", "", "delete key")
	flag.StringVar(&args.History, "history", "", "key-value update history")
	flag.BoolVar(&args.Copy, "cp", false, "copy kv")
	flag.BoolVar(&args.DeepCopy, "dcp", false, "deep copy kv")
	flag.BoolVar(&args.Rename, "rename", false, "rename key")
	flag.BoolVar(&args.GC, "gc", false, "garbage collection")
	flag.Parse()
}

type KV struct {
	Key   string `gorm:"column:key;primary_key"`
	Value string `gorm:"column:value"`
}

type History struct {
	ID        uint64    `gorm:"column:id;primary_key"`
	Key       string    `gorm:"column:key"`
	Value     string    `gorm:"column:value"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

var db *gorm.DB

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	dbfile := path.Join(home, ".show.db")
	db, err = gorm.Open(sqlite.Open(dbfile), &gorm.Config{
		Logger: logger.New(nil, logger.Config{}),
	})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&KV{})
	db.AutoMigrate(&History{})
}

func edit(tx *gorm.DB) error {
	file := path.Join(os.TempDir(), fmt.Sprintf("show-edit-%s", args.Key))
	defer os.Remove(file)

	var kv KV
	if db.Where("key = ?", args.Key).First(&kv).Error == nil {
		if err := ioutil.WriteFile(file, []byte(kv.Value), 0644); err != nil {
			return err
		}
	}

	cmd := exec.Command("nano", file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Run()
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if kv.Value == string(data) {
		return nil
	}
	if err := tx.Save(&KV{Key: args.Key, Value: string(data)}).Error; err != nil {
		return err
	}
	if err := tx.Create(&History{Key: args.Key, Value: string(data), CreatedAt: time.Now()}).Error; err != nil {
		return err
	}
	return nil
}

func doDelete(tx *gorm.DB, key string) error {
	if err := tx.Delete(&KV{Key: key}).Error; err != nil {
		return err
	}
	if err := tx.Where("key = ?", key).Delete(&History{}).Error; err != nil {
		return err
	}
	return nil
}

func showHistory() {
	var histories []History
	db.Where("key = ?", args.History).Order("id DESC").Find(&histories)
	for _, h := range histories {
		fmt.Println(h.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println(h.Value)
	}
}

func doCopy(tx *gorm.DB, src, dist string, deep bool) error {
	var kv KV
	if err := tx.Where("key = ?", src).First(&kv).Error; err != nil {
		return err
	}
	kv.Key = dist
	if err := tx.Create(&kv).Error; err != nil {
		return err
	}
	if deep {
		var histories []History
		tx.Where("key = ?", src).Find(&histories)
		for _, history := range histories {
			history.ID = 0
			history.Key = dist
			if err := tx.Create(&history).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func doRename(tx *gorm.DB, src, dist string) error {
	if err := doCopy(tx, src, dist, true); err != nil {
		return err
	}
	if err := doDelete(tx, src); err != nil {
		return err
	}
	return nil
}

func init() {
	if args.GC {
		if err := db.Exec("VACUUM").Error; err != nil {
			fmt.Println(err.Error())
		}
		os.Exit(0)
	}
	tx := db.Session(&gorm.Session{}).Begin()
	defer tx.Rollback()
	if args.ListAllKeys {
		var kvs []KV
		db.Find(&kvs)
		for _, kv := range kvs {
			var n int64
			db.Model(&History{}).Where("key = ?", kv.Key).Count(&n)
			fmt.Printf("%-50s %5d version\n", kv.Key, n)
		}
		os.Exit(0)
	}
	if args.Key != "" {
		if err := edit(tx); err != nil {
			fmt.Println(err.Error())
		} else {
			tx.Commit()
		}
		os.Exit(0)
	}
	if args.Delete != "" {
		if err := doDelete(tx, args.Delete); err != nil {
			fmt.Println(err.Error())
		} else {
			tx.Commit()
		}
		os.Exit(0)
	}
	if args.History != "" {
		showHistory()
		os.Exit(0)
	}
	if args.Copy || args.DeepCopy && len(os.Args) == 4 {
		if err := doCopy(tx, os.Args[2], os.Args[3], args.DeepCopy); err != nil {
			fmt.Println(err.Error())
		} else {
			tx.Commit()
		}
		os.Exit(0)
	}
	if args.Rename && len(os.Args) == 4 {
		if err := doRename(tx, os.Args[2], os.Args[3]); err != nil {
			fmt.Println(err.Error())
		} else {
			tx.Commit()
		}
		os.Exit(0)
	}
}

func main() {
	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		var kv KV
		if db.Where("key = ?", arg).First(&kv).Error == nil {
			fmt.Println(kv.Value)
		}
	}
}
