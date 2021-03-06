package cmd

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/boltdb/bolt"
	kitlog "github.com/go-kit/kit/log"
	"github.com/gorilla/handlers"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xescugc/rebost/boltdb"
	"github.com/xescugc/rebost/config"
	"github.com/xescugc/rebost/fs"
	"github.com/xescugc/rebost/membership"
	"github.com/xescugc/rebost/storing"
	"github.com/xescugc/rebost/volume"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Starts the rebost server",
		Long:  "Starts the rebost server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.New(viper.GetViper())
			if err != nil {
				return err
			}
			logger := kitlog.NewLogfmtLogger(kitlog.NewSyncWriter(os.Stdout))
			logger = kitlog.With(logger, "ts", kitlog.DefaultTimestampUTC, "caller", kitlog.DefaultCaller)

			if len(cfg.Volumes) == 0 {
				return errors.New("at last one volume is required")
			}
			if cfg.MemberlistName == "" {
				return errors.New("the 'memberlist-name' is required")
			}

			osfs := afero.NewOsFs()

			vs := make([]volume.Local, 0, len(cfg.Volumes))
			for _, vp := range cfg.Volumes {
				bdb, err := createDB(vp)
				if err != nil {
					return fmt.Errorf("error creating the BoltDB: %s", err)
				}
				files, err := boltdb.NewFileRepository(bdb)
				if err != nil {
					return fmt.Errorf("error creating File Repository: %s", err)
				}
				idxkeys, err := boltdb.NewIDXKeyRepository(bdb)
				if err != nil {
					return fmt.Errorf("error creating IDXKeys Repository: %s", err)
				}
				idxvolumes, err := boltdb.NewIDXVolumeRepository(bdb)
				if err != nil {
					return fmt.Errorf("error creating IDXVolumes Repository: %s", err)
				}
				replicas, err := boltdb.NewReplicaRepository(bdb)
				if err != nil {
					return fmt.Errorf("error creating Replica Repository: %s", err)
				}
				suow := fs.UOWWithFs(boltdb.NewUOW(bdb))

				v, err := volume.New(vp, files, idxkeys, idxvolumes, replicas, osfs, suow)
				if err != nil {
					return fmt.Errorf("error creating Volume: %s", err)
				}

				logger.Log("msg", fmt.Sprintf("Attached to volume: %q", vp))
				vs = append(vs, v)
			}

			m, err := membership.New(cfg, vs, cfg.Remote, logger)
			if err != nil {
				return err
			}

			s := storing.New(cfg, m, logger)

			mux := http.NewServeMux()

			mux.Handle("/", storing.MakeHandler(s))

			http.Handle("/", handlers.CustomLoggingHandler(os.Stdout, mux, func(writer io.Writer, params handlers.LogFormatterParams) {
				username := "-"
				if params.URL.User != nil {
					if name := params.URL.User.Username(); name != "" {
						username = name
					}
				}

				host, _, err := net.SplitHostPort(params.Request.RemoteAddr)
				if err != nil {
					host = params.Request.RemoteAddr
				}

				uri := params.Request.RequestURI

				// Requests using the CONNECT method over HTTP/2.0 must use
				// the authority field (aka r.Host) to identify the target.
				// Refer: https://httpwg.github.io/specs/rfc7540.html#CONNECT
				if params.Request.ProtoMajor == 2 && params.Request.Method == "CONNECT" {
					uri = params.Request.Host
				}
				if uri == "" {
					uri = params.URL.RequestURI()
				}
				logger.Log(
					"name", cfg.MemberlistName,
					"host", host,
					"username", username,
					"method", params.Request.Method,
					"uri", uri,
					"status", strconv.Itoa(params.StatusCode),
					"size", strconv.Itoa(params.Size),
				)
			}))

			logger.Log("port", cfg.Port, "msg", "started server")
			return http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil)
		},
	}
)

func createDB(p string) (*bolt.DB, error) {
	db, err := bolt.Open(path.Join(p, "my.db"), 0600, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func init() {
	serveCmd.PersistentFlags().StringP("port", "p", "3805", "Destination port")
	viper.BindPFlag("port", serveCmd.PersistentFlags().Lookup("port"))

	serveCmd.PersistentFlags().StringSliceP("volumes", "v", []string{}, "Volumes to store the data")
	viper.BindPFlag("volumes", serveCmd.PersistentFlags().Lookup("volumes"))

	serveCmd.PersistentFlags().StringP("remote", "r", "", "The URL of a remote Node to join on the cluster")
	viper.BindPFlag("remote", serveCmd.PersistentFlags().Lookup("remote"))

	serveCmd.PersistentFlags().Int("replica", config.DefaultReplica, "The default number of replicas used if none specified on the requests")
	viper.BindPFlag("replica", serveCmd.PersistentFlags().Lookup("replica"))

	serveCmd.PersistentFlags().Int("memberlist-bind-port", 0, "The port is used for both UDP and TCP gossip. By default a free port will be used")
	viper.BindPFlag("memberlist-bind-port", serveCmd.PersistentFlags().Lookup("memberlist-bind-port"))

	serveCmd.PersistentFlags().String("memberlist-name", "", "The name of this node. This must be unique in the cluster.")
	viper.BindPFlag("memberlist-name", serveCmd.PersistentFlags().Lookup("memberlist-name"))

	RootCmd.AddCommand(serveCmd)
}
