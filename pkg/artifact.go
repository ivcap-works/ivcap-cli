package client

import (
	"context"
	"encoding/base64"
	"fmt"
	api "github.com/reinventingscience/ivcap-core-api/http/artifact"
	"io"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListArtifactRequest struct {
	Offset int
	Limit  int
}

func ListArtifacts(ctxt context.Context, cmd *ListArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListArtifactsRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	pyl.AsType(&list)
	return &list, nil
}

func ListArtifactsRaw(ctxt context.Context, cmd *ListArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := artifactPath(nil, adpt)

	pa := []string{}
	if cmd.Offset > 0 {
		pa = append(pa, "offset="+url.QueryEscape(strconv.Itoa(cmd.Offset)))
	}
	if cmd.Limit > 0 {
		pa = append(pa, "limit="+url.QueryEscape(strconv.Itoa(cmd.Limit)))
	}
	if len(pa) > 0 {
		path = path + "?" + strings.Join(pa, "&")
	}
	//fmt.Printf("PATH: %s\n", path)
	return (*adpt).Get(ctxt, path, logger)
}

// /**** CREATE ****/

type CreateArtifactRequest struct {
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	Collection string            `json:"collection"`
	Meta       map[string]string `json:"meta"`
}

func CreateArtifact(
	ctxt context.Context,
	cmd *CreateArtifactRequest,
	contentType string,
	reader io.Reader,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.UploadResponseBody, error) {
	if res, err := CreateArtifactRaw(ctxt, cmd, contentType, reader, adpt, logger); err == nil {
		var artifact api.UploadResponseBody
		if err := res.AsType(&artifact); err != nil {
			return nil, err
		}
		return &artifact, nil
	} else {
		return nil, err
	}
}

func UploadArtifact(
	ctxt context.Context,
	reader io.Reader,
	size int64,
	offset int64,
	chunkSize int64,
	path string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (err error) {
	if offset > 0 {
		switch r := reader.(type) {
		case io.Seeker:
			r.Seek(offset, io.SeekCurrent)
		default:
			io.CopyN(ioutil.Discard, r, offset)
		}
	}

	remaining := size - offset
	fragSize := chunkSize
	if fragSize < 0 {
		fragSize = remaining // no chunking
	}
	reader = AddProgressBar("... uploading file", remaining, reader)
	// var pyld adapter.Payload
	for remaining > 0 {
		psize := remaining
		if psize > fragSize {
			psize = fragSize
		}
		off := size - remaining
		r := &io.LimitedReader{R: reader, N: psize}
		h := map[string]string{
			"Content-Type":  "application/offset+octet-stream",
			"Upload-Offset": fmt.Sprintf("%d", off),
			"Tus-Resumable": "1.0.0",
		}
		// var pyld adapter.Payload
		_, err = (*adpt).Patch(context.Background(), path, r, psize, &h, logger)
		if err != nil {
			fmt.Printf("\n") // To move past progress bar
			return
		}
		remaining -= psize - r.N
	}
	fmt.Printf("\n") // To move past progress bar
	return
}

func AddProgressBar(description string, size int64, reader io.Reader) io.Reader {
	bar := GetProgressBar(description, size)
	return io.TeeReader(reader, bar)
}

func GetProgressBar(description string, size int64) io.Writer {
	return progressbar.NewOptions64(size,
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func CreateArtifactRaw(
	ctxt context.Context,
	cmd *CreateArtifactRequest,
	contentType string,
	reader io.Reader,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(nil, adpt)
	headers := make(map[string]string)
	contentLength := cmd.Size
	if reader == nil {
		headers["X-Content-Type"] = contentType
		headers["Upload-Length"] = fmt.Sprintf("%d", cmd.Size)
		headers["Tus-Resumable"] = "1.0.0"
		contentLength = 0
	} else {
		headers["Content-Type"] = contentType
	}
	if cmd.Name != "" {
		headers["X-Name"] = BaseEncode(cmd.Name)
	}
	if cmd.Collection != "" {
		headers["X-Collection"] = BaseEncode(cmd.Collection)
	}
	var meta []string
	for key, value := range cmd.Meta {
		k := BaseEncode(key)
		v := BaseEncode(value)
		meta = append(meta, fmt.Sprintf("%s %s", k, v))
	}
	if len(meta) > 0 {
		headers["Upload-Metadata"] = cmd.Name
	}
	return (*adpt).Post(ctxt, path, reader, contentLength, &headers, logger)
}

func BaseEncode(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

/**** READ ****/

type ReadArtifactRequest struct {
	Id string
}

func ReadArtifact(ctxt context.Context, cmd *ReadArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	if res, err := ReadArtifactRaw(ctxt, cmd, adpt, logger); err == nil {
		var artifact api.ReadResponseBody
		if err := res.AsType(&artifact); err != nil {
			return nil, err
		}
		return &artifact, nil
	} else {
		return nil, err
	}
}

func ReadArtifactRaw(ctxt context.Context, cmd *ReadArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := artifactPath(&cmd.Id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

/**** COLLECTION ****/

func AddArtifactToCollection(
	ctxt context.Context,
	artifactID string,
	collectionName string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.collections/%s", path, url.PathEscape(collectionName))
	return (*adpt).Put(ctxt, path, nil, -1, nil, logger)
}

func RemoveArtifactToCollection(
	ctxt context.Context,
	artifactID string,
	collectionName string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.collections/%s", path, url.PathEscape(collectionName))
	return (*adpt).Delete(ctxt, path, logger)
}

/**** METADATA ****/

func AddArtifactMeta(
	ctxt context.Context,
	artifactID string,
	schemaName string,
	schema io.Reader,
	size int64,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.metadata/%s", path, url.PathEscape(schemaName))
	return (*adpt).Put(ctxt, path, schema, size, nil, logger)
}

/**** UTILS ****/

func artifactPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/artifacts"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
