package cmd

import (
	"fmt"
	"os"

	"github.com/alec-rabold/zipspy/pkg/zipfile"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var files, outFiles []string
var bucket, key, outFile string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract one or more files from  S3 zip archive",
	Long: `Downloads range(s) of bytes from S3 zip archive
	containing the compressed file(s), the decompresses the data.
	
	ex: 
	zipspy extract -b myBucket -k myKey -f plan.txt
	zipspy extract -b myBucket -k myKey -f plan.txt -o my/directory/plan.txt
	zipspy extract -b myBucket -k myKey -f plan1.txt, plan2.txt, path/to/plan3.txt, /directory
	zipspy extract -b myBucket -k myKey -f plan1.txt -o plan1.txt -f plan2.txt -o plan2.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(files) == 0 || bucket == "" || key == "" {
			cmd.Usage()
			os.Exit(1)
		}
		if len(outFiles) > 1 && (len(outFiles) != len(files)) {
			cmd.Usage()
			log.Error("error: must specify one output file for every search term")
			os.Exit(1)
		}
		z := zipfile.NewFileExtractor(bucket, key)
		records, err := z.ExtractFiles(files)
		if err != nil {
			log.Errorf("error extracting files from archive, err: %v", err)
			return err
		}
		if len(outFiles) == 0 {
			for _, v := range records.FileMap {
				for _, f := range v {
					fmt.Println(f.Contents.String())
				}
			}
		} else if len(outFiles) == 1 {
			f, err := os.OpenFile(outFiles[0], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Errorf("error opening file (name: %s), err: %v", outFiles[0], err)
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					log.Errorf("error closing file (name: %s), err: %v", outFiles[0], err)
					panic(err)
				}
			}()
			for _, files := range records.FileMap {
				for _, file := range files {
					if _, err := f.Write(file.Contents.Bytes()); err != nil {
						log.Errorf("error writing to file (name: %s), err: %v", outFiles[0], err)
						panic(err)
					}
				}
			}
		} else if len(outFiles) > 1 {
			outputMap := make(map[string]string) // searchTerm -> outputFile
			for i := range outFiles {
				outputMap[files[i]] = outFiles[i]
			}
			for searchTerm, files := range records.FileMap {
				f, err := os.OpenFile(outputMap[searchTerm], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Errorf("error opening file (name: %s), err: %v", outFiles[0], err)
					return err
				}
				defer func() {
					if err := f.Close(); err != nil {
						log.Errorf("error closing file (name: %s), err: %v", outFiles[0], err)
						panic(err)
					}
				}()
				for _, file := range files {
					if _, err := f.Write(file.Contents.Bytes()); err != nil {
						log.Errorf("error writing to file (name: %s), err: %v", outFiles[0], err)
						panic(err)
					}
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.PersistentFlags().StringVarP(&key, "key", "k", "", "(required) name of the S3 key (object)")
	extractCmd.PersistentFlags().StringVarP(&bucket, "bucket", "b", "", "(required) name of the S3 bucket")
	extractCmd.PersistentFlags().StringSliceVarP(&outFiles, "out", "o", []string{}, "name(s) of the file(s) to write output to")
	extractCmd.PersistentFlags().StringSliceVarP(&files, "file", "f", []string{}, "(required) names of the files/paths to extract (e.g. plan.txt, /path/to/plan.txt, /directory)")
}
