package layerdb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic-io/mindb"
	"github.com/klauspost/compress/zstd"
	"github.com/model-ci/apack/internal/log"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"go.etcd.io/bbolt"
)

type DB interface {
	OCI
	Raw(opt *Options) (RAW, error)
	GC(ctx context.Context) error
	Close() error
}

type OCI interface {
	Index(ctx context.Context, reference string, desc oci.Descriptor) error
	Manifest(ctx context.Context, reference string) (oci.Manifest, oci.Descriptor, error)
	Manifests(ctx context.Context) ([]oci.Manifest, []string, error)
	Copy(ctx context.Context, ref1, ref2 string) error
	Read(ctx context.Context, desc oci.Descriptor) (io.ReadCloser, error)
	Write(ctx context.Context, desc oci.Descriptor, content io.ReadCloser, progress io.Writer) error
	Set(ctx context.Context, desc oci.Descriptor, content io.ReadCloser) error
	Layering(ctx context.Context, mediaType string, content, snap *os.File, progress io.Writer) (oci.Descriptor, digest.Digest, error)
	Contenting(ctx context.Context, mediaType string, content *os.File, progress io.Writer) (io.ReadCloser, error)
	Content(ctx context.Context, desc oci.Descriptor, diffid digest.Digest, content io.ReadCloser) (io.Reader, error)
	Delete(ctx context.Context, reference string) error
	Snap(ctx context.Context, reference string) error
	Purge(ctx context.Context, reference string) error
	Exists(ctx context.Context, desc oci.Descriptor) (bool, error)
}

type RAW interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) ([]byte, error)
	PutStream(ctx context.Context, key string, data io.ReadCloser) error
	GetStream(ctx context.Context, key string) (io.ReadCloser, error)
	ForEach(ctx context.Context, fn func(key, value []byte) error) error
}

type db struct {
	path         string
	metadataPath string
	dataPath     string
	bucket       string
	bolt         *bbolt.DB
	min          *mindb.DB
	kv           *kv
	options      *Options
}

type Options struct {
	GCOption
}

func Open(path string, options *Options) (DB, error) {
	db := &db{path: path, options: options}
	db.metadataPath = filepath.Join(path, Metadata)
	db.dataPath = filepath.Join(path, Blobs)
	db.bucket = LayerDir

	var err error

	db.bolt, err = bbolt.Open(db.metadataPath, 0600, &bbolt.Options{})
	if err != nil {
		return nil, err
	}

	db.kv, err = newKV(db.bolt, IndexDir)
	if err != nil {
		return nil, err
	}

	db.min, err = mindb.New(db.dataPath)
	if err != nil {
		return nil, err
	}

	return db, db.init()
}

func (r *db) init() error {
	if exists, err := r.kv.Exists([]byte(OCIImageLayoutFile)); err != nil {
		return err
	} else if exists {
		if idxExists, err := r.kv.Exists([]byte(OCIImageIndexFile)); err != nil {
			return err
		} else if idxExists {
			return nil
		}
	}

	layout := &OCILayout{
		MediaType: OCILayoutMediaType,
	}
	layout.Version = OCIImageLayoutVersion
	layoutJson, err := layout.MarshalJSON()
	if err != nil {
		return err
	}

	err = r.kv.Put([]byte(OCIImageLayoutFile), layoutJson)
	if err != nil {
		return err
	}

	if exists, err := r.kv.Exists([]byte(OCIImageIndexFile)); err != nil {
		return err
	} else if !exists {
		idx := &Index{}
		idx.SchemaVersion = OCIImageIndexVersion
		idx.MediaType = OCIImageIndexMediaType
		idxJson, err := idx.MarshalJSON()
		if err != nil {
			return err
		}

		err = r.kv.Put([]byte(OCIImageIndexFile), idxJson)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *db) Index(ctx context.Context, reference string, desc oci.Descriptor) error {
	return r.index(ctx, reference, desc)
}

func (r *db) Copy(ctx context.Context, ref1, ref2 string) error {
	idxJson, err := r.kv.Get([]byte(OCIImageIndexFile))
	if err != nil {
		return err
	}

	idx := &Index{}
	if err = idx.UnmarshalJSON(idxJson); err != nil {
		return err
	}

	var manifest oci.Descriptor
	found := false
	for i, item := range idx.Manifests {
		if v, ok := item.Annotations[OCIAnnotationRefName]; ok {
			if v == ref1 {
				found = true
				manifest = DeepCopyDescriptor(idx.Manifests[i])
			}
		}
	}

	if !found {
		return fmt.Errorf("manifest not found for reference: %s", ref1)
	}

	manifest.Annotations[OCIAnnotationRefName] = ref2
	idx.Manifests = append(idx.Manifests, manifest)

	idxJson, err = idx.MarshalJSON()
	if err != nil {
		return err
	}

	err = r.kv.Put([]byte(OCIImageIndexFile), idxJson)
	if err != nil {
		return err
	}

	return nil
}

func (r *db) index(ctx context.Context, reference string, desc oci.Descriptor) error {
	idxJson, err := r.kv.Get([]byte(OCIImageIndexFile))
	if err != nil {
		return err
	}

	idx := &Index{}
	if err = idx.UnmarshalJSON(idxJson); err != nil {
		return err
	}

	annots := map[string]string{
		OCIAnnotationRefName: reference,
	}

	found := false
	for i, item := range idx.Manifests {
		if v, ok := item.Annotations[OCIAnnotationRefName]; ok {
			if v == reference {
				found = true
				desc.Annotations = annots
				idx.Manifests[i] = desc
			}
		}
	}

	if !found {
		desc.Annotations = annots
		idx.Manifests = append(idx.Manifests, desc)
	}

	idxJson, err = idx.MarshalJSON()
	if err != nil {
		return err
	}

	err = r.kv.Put([]byte(OCIImageIndexFile), idxJson)
	if err != nil {
		return err
	}

	return nil
}

func (r *db) Manifest(ctx context.Context, reference string) (oci.Manifest, oci.Descriptor, error) {
	return r.manifest(ctx, reference)
}

func (r *db) manifest(ctx context.Context, reference string) (oci.Manifest, oci.Descriptor, error) {
	manifests, descs, _, err := r.manifests(ctx, reference)
	if err != nil {
		return oci.Manifest{}, oci.DescriptorEmptyJSON, err
	}
	if len(manifests) == 0 {
		return oci.Manifest{}, oci.DescriptorEmptyJSON, fmt.Errorf("no manifest found for reference: %s", reference)
	}
	if len(manifests) > 1 && reference != "" {
		return oci.Manifest{}, oci.DescriptorEmptyJSON, fmt.Errorf("multiple manifests found for reference %s", reference)
	}
	return manifests[0], descs[0], nil
}

func (r *db) Manifests(ctx context.Context) ([]oci.Manifest, []string, error) {
	manifests, _, refs, err := r.manifests(ctx, "")
	return manifests, refs, err
}

func (r *db) manifests(ctx context.Context, reference string) ([]oci.Manifest, []oci.Descriptor, []string, error) {
	idxJson, err := r.kv.Get([]byte(OCIImageIndexFile))
	if err != nil {
		return nil, nil, nil, err
	}

	idx := &Index{}
	if err = idx.UnmarshalJSON(idxJson); err != nil {
		return nil, nil, nil, err
	}

	var manifests []oci.Manifest
	var descs []oci.Descriptor
	var refs []string
	for _, desc := range idx.Manifests {
		manifestBytes, err := r.kv.Get([]byte(blobKey(desc)))
		if err != nil {
			return nil, nil, nil, err
		}

		mf := &Manifest{}
		if err = mf.UnmarshalJSON(manifestBytes); err != nil {
			return nil, nil, nil, err
		}

		if ref, ok := desc.Annotations[OCIAnnotationRefName]; ok {
			manifests = append(manifests, mf.Manifest)
			descs = append(descs, desc)
			refs = append(refs, ref)
			if ref == reference {
				l := len(manifests)
				return manifests[l-1:], descs[l-1:], refs[l-1:], nil
			}
		}
	}
	return manifests, descs, refs, nil
}

func ReaderToBytes(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

func BytesToReadCloser(data []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(data))
}

func (r *db) Read(ctx context.Context, desc oci.Descriptor) (io.ReadCloser, error) {
	ti := ParseMediaType(desc.MediaType)
	if ti.IsTar {
		blob, _, err := r.min.GetObjectStream(r.bucket, blobKey(desc))
		if err != nil {
			return nil, err
		}
		return blob, nil
	}

	bytes, err := r.kv.Get([]byte(blobKey(desc)))
	if err != nil {
		return nil, err
	}
	return BytesToReadCloser(bytes), nil
}

func (r *db) Write(ctx context.Context, desc oci.Descriptor, content io.ReadCloser, progress io.Writer) error {
	ti := ParseMediaType(desc.MediaType)
	if ti.IsTar {
		_, err := r.min.PutObjectStream(r.bucket, blobKey(desc), content, -1, DefaultMediaType, nil, progress)
		return err
	}

	return r.Set(ctx, desc, content)
}

func (r *db) Set(ctx context.Context, desc oci.Descriptor, content io.ReadCloser) error {
	bytes, err := ReaderToBytes(content)
	if err != nil {
		return err
	}

	err = r.kv.Put([]byte(blobKey(desc)), bytes)
	if err != nil {
		return err
	}

	return nil
}

func (r *db) Layering(ctx context.Context, mediaType string, content, snap *os.File, progress io.Writer) (oci.Descriptor, digest.Digest, error) {
	ti := ParseMediaType(mediaType)
	return r.makeContentLayer(content, snap, ti.Algo, mediaType, progress)
}

func (r *db) makeContentLayer(content, snap *os.File, comp Algorithm, mediaType string, progress io.Writer) (oci.Descriptor, digest.Digest, error) {
	return r.compressLayerV2(content, comp, mediaType, io.Discard, snap, progress)
}

func (r *db) Contenting(ctx context.Context, mediaType string, content *os.File, progress io.Writer) (io.ReadCloser, error) {
	ti := ParseMediaType(mediaType)

	pr, pw := io.Pipe()

	var compressErr error

	go func() {
		defer pw.Close()
		_, _, compressErr = r.compressLayerV2(content, ti.Algo, mediaType, pw, nil, progress)
		if compressErr != nil {
			pw.CloseWithError(compressErr)
		}
	}()

	return pr, nil
}

func (r *db) saveContentLayer(content *os.File, comp Algorithm, mediaType string, progress io.Writer) (oci.Descriptor, digest.Digest, error) {
	pr, pw := io.Pipe()

	var desc oci.Descriptor
	var diffid digest.Digest
	var compressErr error

	go func() {
		defer pw.Close()
		desc, diffid, compressErr = r.compressLayerV2(content, comp, mediaType, pw, nil, progress)
		if compressErr != nil {
			pw.CloseWithError(compressErr)
		}
	}()

	tmpBlobKey := strconv.FormatInt(int64(content.Fd()), 10) + "-" + strconv.FormatInt(time.Now().UnixMicro(), 10)

	_, err := r.min.PutObjectStream(r.bucket, tmpBlobKey, pr, -1, DefaultMediaType, nil, nil)
	if err != nil {
		pr.Close()
		return oci.DescriptorEmptyJSON, diffid, err
	}

	if compressErr != nil {
		return oci.DescriptorEmptyJSON, diffid, compressErr
	}

	exist, _ := r.min.ObjectExists(r.bucket, blobKey(desc))
	if exist {
		// TODO: tmpBlobKey GC
		return desc, diffid, nil
	}

	log.Logger.Debugf("start rename object: %s->%s", tmpBlobKey, blobKey(desc))

	return desc, diffid, r.min.RenameObject(r.bucket, tmpBlobKey, blobKey(desc))
}

func (r *db) compressLayerV2(f *os.File, comp Algorithm, mediaType string, output, rawOutput, progress io.Writer) (oci.Descriptor, digest.Digest, error) {
	compressedDigester := digest.Canonical.Digester()
	var diffIdDigester digest.Digester

	compressedCounter := &countingWriter{Writer: output}

	var compressedWriter io.WriteCloser
	var tarWriter *tar.Writer
	var err error

	// data link: file -> raw-output -> tar -> compress -> hash -> count -> output
	hashWriter := io.MultiWriter(compressedCounter, compressedDigester.Hash())

	switch comp {
	case Gzip:
		compressedWriter = gzip.NewWriter(hashWriter)
		diffIdDigester = digest.Canonical.Digester()
		tarHashWriter := io.MultiWriter(compressedWriter, diffIdDigester.Hash())
		tarWriter = tar.NewWriter(tarHashWriter)
	case GzipFastest:
		compressedWriter, err = gzip.NewWriterLevel(hashWriter, gzip.BestSpeed)
		if err != nil {
			return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), fmt.Errorf("failed to set up gzip compression: %w", err)
		}
		diffIdDigester = digest.Canonical.Digester()
		tarHashWriter := io.MultiWriter(compressedWriter, diffIdDigester.Hash())
		tarWriter = tar.NewWriter(tarHashWriter)
	case Zstd:
		compressedWriter, err = zstd.NewWriter(hashWriter, zstd.WithEncoderLevel(zstd.SpeedFastest))
		if err != nil {
			return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), fmt.Errorf("failed to set up zstd compression: %w", err)
		}
		diffIdDigester = digest.Canonical.Digester()
		tarHashWriter := io.MultiWriter(compressedWriter, diffIdDigester.Hash())
		tarWriter = tar.NewWriter(tarHashWriter)
	case None:
		diffIdDigester = digest.Canonical.Digester()
		tarHashWriter := io.MultiWriter(hashWriter, diffIdDigester.Hash())
		tarWriter = tar.NewWriter(tarHashWriter)
	}

	fi, err := f.Stat()
	if err != nil {
		return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), err
	}

	var progressWriter io.Writer = tarWriter
	if rawOutput != nil && progress != nil {
		progressWriter = io.MultiWriter(tarWriter, rawOutput, progress)
	} else if rawOutput != nil {
		progressWriter = io.MultiWriter(tarWriter, rawOutput)
	} else if progress != nil {
		progressWriter = io.MultiWriter(tarWriter, progress)
	}
	// else: progressWriter = tarWriter (默认)

	err = r.streamFileToTar(fi, f, tarWriter, progressWriter)
	if err != nil {
		return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), err
	}

	if err := tarWriter.Close(); err != nil {
		return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), fmt.Errorf("failed to close tar writer: %w", err)
	}

	if compressedWriter != nil {
		if err := compressedWriter.Close(); err != nil {
			return oci.DescriptorEmptyJSON, diffIdDigester.Digest(), fmt.Errorf("failed to close compressed writer: %w", err)
		}
	}

	desc := oci.Descriptor{
		MediaType:   mediaType,
		Digest:      compressedDigester.Digest(),
		Size:        compressedCounter.count,
		Annotations: map[string]string{},
	}

	return desc, diffIdDigester.Digest(), nil
}

type countingWriter struct {
	io.Writer
	count int64
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.Writer.Write(p)
	cw.count += int64(n)
	return
}

func (r *db) streamFileToTar(fi fs.FileInfo, f io.ReadCloser, tw *tar.Writer, pw io.Writer) error {
	if err := writeHeaderToTar(fi, tw); err != nil {
		return err
	}

	const bufferSize = 64 * 1024
	buffer := make([]byte, bufferSize)

	var totalWritten int64
	for {
		n, readErr := f.Read(buffer)
		if n > 0 {
			written, writeErr := pw.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write to tar: %w", writeErr)
			}
			totalWritten += int64(written)
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("failed to read file: %w", readErr)
		}
	}

	if totalWritten != fi.Size() {
		return fmt.Errorf("incomplete file write: expected %d bytes, wrote %d bytes", fi.Size(), totalWritten)
	}

	return nil
}

func (r *db) Exists(ctx context.Context, desc oci.Descriptor) (bool, error) {
	exist, err := r.kv.Exists([]byte(blobKey(desc)))
	if !exist {
		return r.min.ObjectExists(r.bucket, blobKey(desc))
	}
	return exist, err
}

func ManifestDesc(configDesc, subjectDesc oci.Descriptor, layerDescs []oci.Descriptor, mediaType string) (oci.Descriptor, []byte, error) {
	manifest := &Manifest{
		Manifest: oci.Manifest{
			Versioned:   specs.Versioned{SchemaVersion: OCIImageIndexVersion},
			MediaType:   mediaType,
			Config:      configDesc,
			Layers:      layerDescs,
			Annotations: map[string]string{},
		},
	}

	bytes, err := manifest.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, nil, err
	}

	desc := oci.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(bytes),
		Size:      int64(len(bytes)),
	}

	return desc, bytes, nil
}

func ConfigDesc(bytes []byte, mediaType string) oci.Descriptor {
	return oci.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(bytes),
		Size:      int64(len(bytes)),
	}
}

func writeHeaderToTar(fi fs.FileInfo, tw *tar.Writer) error {
	header, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return fmt.Errorf("failed to generate header for %s: %w", fi.Name(), err)
	}
	sanitizeTarHeader(header)
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	return nil
}

func sanitizeTarHeader(header *tar.Header) {
	header.Name = filepath.ToSlash(header.Name)
	header.Mode = 0o666
	header.AccessTime = time.Time{}
	header.ModTime = time.Time{}
	header.ChangeTime = time.Time{}
	header.Uid = 1000
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
}

func (r *db) Content(ctx context.Context, desc oci.Descriptor, diffid digest.Digest, content io.ReadCloser) (io.Reader, error) {
	var err error

	if content == nil {
		content, _, err = r.min.GetObjectStream(r.bucket, blobKey(desc))
		if err != nil {
			return nil, err
		}
	}

	ti := ParseMediaType(desc.MediaType)
	var reader io.ReadCloser
	switch ti.Algo {
	case Gzip, GzipFastest:
		reader, err = gzip.NewReader(content)
		if err != nil {
			return nil, err
		}
	case Zstd:
		decoder, err := zstd.NewReader(content, nil)
		if err != nil {
			return nil, err
		}
		reader = io.NopCloser(decoder)
	case None:
		reader = content
	}

	return &verifyingReader{
		reader:   reader,
		verifier: diffid.Verifier(),
		diffid:   diffid,
	}, nil
}

type verifyingReader struct {
	reader   io.ReadCloser
	verifier digest.Verifier
	diffid   digest.Digest
	verified bool
}

func (v *verifyingReader) Read(p []byte) (n int, err error) {
	n, err = v.reader.Read(p)
	if n > 0 {
		v.verifier.Write(p[:n])
	}

	if err == io.EOF && !v.verified {
		if !v.verifier.Verified() {
			return n, fmt.Errorf("failed to verify diffid %s: checksum mismatch", v.diffid)
		}
		v.verified = true
	}

	return n, err
}

func (v *verifyingReader) Close() error {
	return v.reader.Close()
}

func (r *db) Delete(ctx context.Context, reference string) error {
	return r.annotation(ctx, reference, OCIAnnotationDeleted, reference, add)
}

func (r *db) Close() error {
	if r.kv != nil {
		if err := r.kv.Close(); err != nil {
			return err
		}
	}
	if r.min != nil {
		if err := r.min.Close(); err != nil {
			return err
		}
	}
	return nil
}

// strconv.FormatInt(time.Now().UnixMilli(), 10)
func (r *db) Snap(ctx context.Context, reference string) error {
	return r.annotation(ctx, reference, OCIAnnotationSnapshotRef, reference, add)
}

type annotationAction int8

const (
	add annotationAction = iota
	del
)

func (r *db) annotation(ctx context.Context, reference, key, value string, action annotationAction) error {
	return r.kv.UpdateInPlace([]byte(OCIImageIndexFile), func(idxJson []byte) ([]byte, error) {
		if idxJson == nil {
			return nil, fmt.Errorf("index file not found")
		}

		idx := &Index{}
		if err := idx.UnmarshalJSON(idxJson); err != nil {
			return nil, err
		}

		for i, manifest := range idx.Manifests {
			if manifest.Annotations[OCIAnnotationRefName] == reference {
				switch action {
				case add:
					manifest.Annotations[key] = value
				case del:
					delete(manifest.Annotations, key)
				}
				idx.Manifests[i] = manifest
				break
			}
		}

		return idx.MarshalJSON()
	})
}

func (r *db) unlink(ctx context.Context, refs []string) error {
	return r.kv.UpdateInPlace([]byte(OCIImageIndexFile), func(idxJson []byte) ([]byte, error) {
		if idxJson == nil {
			return nil, fmt.Errorf("index file not found")
		}

		idx := &Index{}
		if err := idx.UnmarshalJSON(idxJson); err != nil {
			return nil, err
		}

		refsDict := map[string]bool{}
		for _, ref := range refs {
			refsDict[ref] = true
		}

		j := 0
		for i := range idx.Manifests {
			if ref, ok := idx.Manifests[i].Annotations[OCIAnnotationRefName]; !ok || !refsDict[ref] {
				idx.Manifests[j] = idx.Manifests[i]
				j++
			}
		}
		idx.Manifests = idx.Manifests[:j]

		return idx.MarshalJSON()
	})
}

func (r *db) Purge(ctx context.Context, reference string) error {
	return r.annotation(ctx, reference, OCIAnnotationSnapshotRef, "", del)
}

func (r *db) GC(ctx context.Context) error {
	gc := NewGC(r, &GCOption{
		GCPolicy: OnlyDelete,
		DryRun:   false,
	})
	return gc.RunGC(ctx)
}

func (r *db) Raw(opt *Options) (RAW, error) {
	raw := &raw{bucket: RawDir}
	var err error
	raw.kv, err = newKV(r.bolt, raw.bucket)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

type raw struct {
	bucket string
	kv     *kv
	min    *mindb.DB
}

func (r *raw) Put(ctx context.Context, key, value string) error {
	return r.kv.Put([]byte(key), []byte(value))
}

func (r *raw) Get(ctx context.Context, key string) ([]byte, error) {
	return r.kv.Get([]byte(key))
}

func (r *raw) ForEach(ctx context.Context, fn func(key, value []byte) error) error {
	return r.kv.ForEach(fn)
}

func (r *raw) PutStream(ctx context.Context, key string, data io.ReadCloser) error {
	_, err := r.min.PutObjectStream(r.bucket, key, data, -1, DefaultMediaType, nil, nil)
	return err
}

func (r *raw) GetStream(ctx context.Context, key string) (io.ReadCloser, error) {
	data, _, err := r.min.GetObjectStream(r.bucket, key)
	return data, err
}

type kv struct {
	db         *bbolt.DB
	bucketName []byte
}

func newKV(db *bbolt.DB, bucket string) (*kv, error) {
	kv := &kv{
		db:         db,
		bucketName: []byte(bucket),
	}

	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(kv.bucketName)
		return err
	})

	return kv, err
}

func (bw *kv) Put(key, value []byte) error {
	return bw.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		return bucket.Put(key, value)
	})
}

func (bw *kv) Get(key []byte) ([]byte, error) {
	var value []byte
	err := bw.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		value = bucket.Get(key)
		return nil
	})
	return value, err
}

func (bw *kv) UpdateInPlace(key []byte, updateFn func([]byte) ([]byte, error)) error {
	return bw.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		data := bucket.Get(key)

		newData, err := updateFn(data)
		if err != nil {
			return err
		}

		return bucket.Put(key, newData)
	})
}

func (bw *kv) Delete(key []byte) error {
	return bw.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		return bucket.Delete(key)
	})
}

func (bw *kv) DeleteIfExists(key []byte) error {
	return bw.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		if bucket.Get(key) == nil {
			return nil // 键不存在，直接返回成功
		}
		return bucket.Delete(key)
	})
}

func (bw *kv) Exists(key []byte) (bool, error) {
	var exists bool
	err := bw.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		value := bucket.Get(key)
		exists = value != nil
		return nil
	})
	return exists, err
}

func (bw *kv) ForEach(fn func(key, value []byte) error) error {
	return bw.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bw.bucketName)
		if bucket == nil {
			return nil // bucket 不存在
		}

		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			if err := fn(key, value); err != nil {
				return err
			}
		}
		return nil
	})
}

func (bw *kv) Close() error {
	return bw.db.Close()
}

type MediaType struct {
	IsTar        bool
	IsCompressed bool
	Algo         Algorithm
}

func ParseMediaType(mediaType string) MediaType {
	mediaType = strings.ToLower(mediaType)

	tarPattern := regexp.MustCompile(`\.tar($|[\+\.]|\s)|tar$|tar\+\w+`)
	if !tarPattern.MatchString(mediaType) {
		return MediaType{IsTar: false}
	}

	compressions := map[string]*regexp.Regexp{
		string(Gzip): regexp.MustCompile(`(gzip|gz)`),
		string(Zstd): regexp.MustCompile(`zstd`),
	}

	for algo, pattern := range compressions {
		if pattern.MatchString(mediaType) {
			return MediaType{
				IsTar:        true,
				IsCompressed: true,
				Algo:         Algorithm(algo),
			}
		}
	}

	return MediaType{IsTar: true, IsCompressed: false, Algo: Algorithm(None)}
}

type GC struct {
	opt *GCOption
	db  *db
}

type GCOption struct {
	GCPolicy Policy
	DryRun   bool
}

type Policy int8

const (
	OnlyDelete Policy = iota
	ReserveRunning
	CleanOut
)

const (
	contentLayer int8 = iota
	otherLayer
	Layer
)

func NewGC(db *db, opt *GCOption) *GC {
	return &GC{
		opt: opt,
	}
}

func (gc *GC) RunGC(ctx context.Context) error {
	nodes, candidates, refs, err := gc.identifyDeletionCandidates(ctx)
	if err != nil {
		return err
	}

	tm, err := newTriMark(&nodes[contentLayer])
	if err != nil {
		return err
	}
	tm.sweep(candidates)

	return gc.scaling(ctx, nodes[:], refs)
}

func (gc *GC) identifyDeletionCandidates(ctx context.Context) ([][]oci.Descriptor, []oci.Descriptor, []string, error) {
	descs := [Layer][]oci.Descriptor{}

	idxJson, err := gc.db.kv.Get([]byte(OCIImageIndexFile))
	if err != nil {
		return nil, nil, nil, err
	}
	idx := &Index{}
	if err = idx.UnmarshalJSON(idxJson); err != nil {
		return nil, nil, nil, err
	}

	var refs []string
	var candidates []oci.Descriptor
	for _, manifestDesc := range idx.Manifests {
		if ref, rok := manifestDesc.Annotations[OCIAnnotationRefName]; rok {
			manifest, _, err := gc.db.manifest(ctx, ref)
			if err != nil {
				// TODO: log
				continue
			}

			switch gc.opt.GCPolicy {
			case OnlyDelete:
				if _, dok := manifestDesc.Annotations[OCIAnnotationDeleted]; !dok {
					descs[contentLayer] = append(descs[contentLayer], manifest.Layers...)
				} else {
					candidates = append(candidates, manifest.Layers...)
				}
			case ReserveRunning:
				if _, sok := manifestDesc.Annotations[OCIAnnotationSnapshotRef]; sok {
					descs[contentLayer] = append(descs[contentLayer], manifest.Layers...)
				} else {
					candidates = append(candidates, manifest.Layers...)
				}
			case CleanOut:
				candidates = append(candidates, manifest.Layers...)
			}
			descs[otherLayer] = append(descs[otherLayer], manifest.Config, manifestDesc)
			refs = append(refs, ref)
		}
	}

	return descs[:], candidates, refs, nil
}

func (gc *GC) scaling(ctx context.Context, nodes [][]oci.Descriptor, refs []string) error {
	for _, content := range nodes[contentLayer] {
		if err := gc.db.min.DeleteObject(gc.db.bucket, blobKey(content)); err != nil {
			return err
		}
	}
	for _, other := range nodes[otherLayer] {
		if err := gc.db.kv.DeleteIfExists([]byte(blobKey(other))); err != nil {
			return err
		}
	}
	return gc.db.unlink(ctx, refs)
}

type triColor int8

const (
	White triColor = iota
	Gray
	Black
)

type triMark struct {
	pool  map[string]triColor
	nodes *[]oci.Descriptor
}

func newTriMark(nodes *[]oci.Descriptor) (*triMark, error) {
	return &triMark{
		pool:  map[string]triColor{},
		nodes: nodes,
	}, nil
}

func (tm *triMark) sweep(candidates []oci.Descriptor) {
	for _, node := range *tm.nodes {
		tm.pool[blobKey(node)] = Black
	}

	var nodes []oci.Descriptor
	for _, calculate := range candidates {
		if _, ok := tm.pool[blobKey(calculate)]; ok {
			tm.pool[blobKey(calculate)] = Gray
		} else {
			tm.pool[blobKey(calculate)] = White
			nodes = append(nodes, calculate)
		}
	}

	tm.nodes = &nodes
}

func DeepCopyDescriptor(src oci.Descriptor) oci.Descriptor {
	dst := src

	if src.URLs != nil {
		dst.URLs = make([]string, len(src.URLs))
		copy(dst.URLs, src.URLs)
	}

	if src.Annotations != nil {
		dst.Annotations = make(map[string]string)
		for k, v := range src.Annotations {
			dst.Annotations[k] = v
		}
	}

	if src.Data != nil {
		dst.Data = make([]byte, len(src.Data))
		copy(dst.Data, src.Data)
	}

	if src.Platform != nil {
		platformCopy := *src.Platform
		dst.Platform = &platformCopy
	}

	return dst
}
