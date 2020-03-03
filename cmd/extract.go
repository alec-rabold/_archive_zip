package cmd

import (
	"os"

	"github.com/alec-rabold/zipspy/pkg/zipfile"
	"github.com/spf13/cobra"
)

var files []string
var bucket, key string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract one or more files from  S3 zip archive",
	Long: `Downloads range(s) of bytes from S3 zip archive
	containing the compressed file(s), the decompresses the data.
	
	ex: 
	zipspy extract -b myBucket -k myKey -f plan.txt
	zipspy extract -b myBucket -k myKey -f plan1.txt, plan2.txt, path\to\plan3.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(files) == 0 || bucket == "" || key == "" {
			cmd.Usage()
			os.Exit(1)
		}
		z := zipfile.NewFileExtractor(bucket, key)
		z.ExtractFiles(files)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.PersistentFlags().StringVarP(&key, "key", "k", "", "(required) name of the S3 key (object)")
	extractCmd.PersistentFlags().StringVarP(&bucket, "bucket", "b", "", "(required) name of the S3 bucket")
	extractCmd.PersistentFlags().StringSliceVarP(&files, "file", "f", []string{}, "(required) names of the files/paths you wish to extract (e.g. plan.txt, \\path\\to\\plan.txt)")
}
