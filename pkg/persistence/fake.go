package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/dollarshaveclub/acyl/pkg/models"
)

type lockingDataMap struct {
	sync.RWMutex
	// each field below represents a table
	d          map[string]*models.QAEnvironment
	helm       map[string][]models.HelmRelease
	k8s        map[string]*models.KubernetesEnvironment
	elogs      map[uuid.UUID]*models.EventLog
	uisessions map[string]*models.UISession
}

// FakeDataLayer is a fake implementation of DataLayer that persists data in-memory, for testing purposes
type FakeDataLayer struct {
	CreateMissingEventLog bool
	data                  *lockingDataMap
	delay                 time.Duration
}

var _ DataLayer = &FakeDataLayer{}

func NewFakeDataLayer() *FakeDataLayer {
	return &FakeDataLayer{
		data: &lockingDataMap{
			d:          make(map[string]*models.QAEnvironment),
			helm:       make(map[string][]models.HelmRelease),
			k8s:        make(map[string]*models.KubernetesEnvironment),
			elogs:      make(map[uuid.UUID]*models.EventLog),
			uisessions: make(map[string]*models.UISession),
		},
	}
}

// NewPopulatedFakeDataLayer returns a FakeDataLayer populated with the supplied data. Input data is not checked for consistency.
func NewPopulatedFakeDataLayer(qaenvs []models.QAEnvironment, k8senvs []models.KubernetesEnvironment, helmreleases []models.HelmRelease) *FakeDataLayer {
	fd := NewFakeDataLayer()
	for i := range qaenvs {
		fd.data.d[qaenvs[i].Name] = &qaenvs[i]
	}
	for i := range k8senvs {
		fd.data.k8s[k8senvs[i].EnvName] = &k8senvs[i]
	}
	for _, hr := range helmreleases {
		fd.data.helm[hr.EnvName] = append(fd.data.helm[hr.EnvName], hr)
	}
	return fd
}

func NewDelayedFakeDataLayer(delay time.Duration) *FakeDataLayer {
	return &FakeDataLayer{
		data: &lockingDataMap{
			d:     make(map[string]*models.QAEnvironment),
			helm:  make(map[string][]models.HelmRelease),
			k8s:   make(map[string]*models.KubernetesEnvironment),
			elogs: make(map[uuid.UUID]*models.EventLog),
		},
		delay: delay,
	}
}

func (fdl *FakeDataLayer) SetDelay(d time.Duration) {
	fdl.delay = d
}

func (fdl *FakeDataLayer) doDelay() {
	time.Sleep(fdl.delay)
}

// Save writes all data to files in dir and returns the filenames, or error
func (fdl *FakeDataLayer) Save(dir string) ([]string, error) {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	names := make([]string, 4)
	// envs
	b, err := json.Marshal(fdl.data.d)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling envs")
	}
	f, err := os.Create(filepath.Join(dir, "envs.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating envs.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing envs.json")
	}
	names[0] = f.Name()
	// helm
	b, err = json.Marshal(fdl.data.helm)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling helm")
	}
	f, err = os.Create(filepath.Join(dir, "helm.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating helm.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing helm.json")
	}
	names[1] = f.Name()
	// k8s
	b, err = json.Marshal(fdl.data.k8s)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling k8s")
	}
	f, err = os.Create(filepath.Join(dir, "k8s.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating k8s.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing k8s.json")
	}
	names[2] = f.Name()
	// elogs
	b, err = json.Marshal(fdl.data.elogs)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling elogs")
	}
	f, err = os.Create(filepath.Join(dir, "elogs.json"))
	if err != nil {
		return nil, errors.Wrap(err, "error creating elogs.json")
	}
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return nil, errors.Wrap(err, "error writing elogs.json")
	}
	names[3] = f.Name()
	return names, nil
}

// Load reads JSON files in dir and loads them, overwriting any existing data
func (fdl *FakeDataLayer) Load(dir string) error {
	fdl.data.Lock()
	defer fdl.data.Unlock()
	b, err := ioutil.ReadFile(filepath.Join(dir, "envs.json"))
	if err != nil {
		return errors.Wrap(err, "error reading envs.json")
	}
	if err := json.Unmarshal(b, &fdl.data.d); err != nil {
		return errors.Wrap(err, "error unmarshaling envs.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "helm.json"))
	if err != nil {
		return errors.Wrap(err, "error reading helm.json")
	}
	if err := json.Unmarshal(b, &fdl.data.helm); err != nil {
		return errors.Wrap(err, "error unmarshaling helm.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "k8s.json"))
	if err != nil {
		return errors.Wrap(err, "error reading k8s.json")
	}
	if err := json.Unmarshal(b, &fdl.data.k8s); err != nil {
		return errors.Wrap(err, "error unmarshaling k8s.json")
	}
	b, err = ioutil.ReadFile(filepath.Join(dir, "elogs.json"))
	if err != nil {
		return errors.Wrap(err, "error reading elogs.json")
	}
	if err := json.Unmarshal(b, &fdl.data.elogs); err != nil {
		return errors.Wrap(err, "error unmarshaling elogs.json")
	}
	return nil
}

func (fdl *FakeDataLayer) CreateQAEnvironment(ctx context.Context, qa *QAEnvironment) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	fdl.data.d[qa.Name] = qa
	fdl.data.Unlock()
	return nil
}

func (fdl *FakeDataLayer) GetQAEnvironment(ctx context.Context, name string) (*QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	fdl.doDelay()
	if qa, ok := fdl.data.d[name]; ok {
		return qa, nil
	}
	return nil, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentConsistently(ctx context.Context, name string) (*QAEnvironment, error) {
	return fdl.GetQAEnvironment(ctx, name)
}

func (fdl *FakeDataLayer) GetQAEnvironments(ctx context.Context) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		out = append(out, *v)
	}
	return out, nil
}

func (fdl *FakeDataLayer) DeleteQAEnvironment(ctx context.Context, name string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		if v.Status != models.Destroyed {
			return errors.New("status must be Destroyed")
		}
		delete(fdl.data.d, name)
	} else {
		return errors.New("env not found")
	}
	if _, ok := fdl.data.k8s[name]; ok {
		delete(fdl.data.k8s, name)
	}
	if _, ok := fdl.data.helm[name]; ok {
		delete(fdl.data.helm, name)
	}
	return nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByStatus(ctx context.Context, status string) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	s, err := models.EnvironmentStatusFromString(status)
	if err != nil {
		return nil, errors.Wrap(err, "bad status")
	}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Status == s {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetRunningQAEnvironments(ctx context.Context) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out1, _ := fdl.GetQAEnvironmentsByStatus(ctx, "Success")
	out2, _ := fdl.GetQAEnvironmentsByStatus(ctx, "Spawned")
	out3, _ := fdl.GetQAEnvironmentsByStatus(ctx, "Updating")
	out := append(out1, append(out2, out3...)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByRepoAndPR(ctx context.Context, repo string, pr uint) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo && v.PullRequest == pr {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByRepo(ctx context.Context, repo string) ([]QAEnvironment, error) {
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentBySourceSHA(ctx context.Context, sourceSHA string) (*QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceSHA == sourceSHA {
			return v, nil
		}
	}
	return nil, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsBySourceBranch(ctx context.Context, sourceBranch string) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceBranch == sourceBranch {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetQAEnvironmentsByUser(ctx context.Context, user string) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.User == user {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetExtantQAEnvironments(ctx context.Context, repo string, pr uint) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.Repo == repo && v.PullRequest == pr && v.Status != models.Destroyed && v.Status != models.Failure {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) SetQAEnvironmentStatus(ctx context.Context, name string, status EnvironmentStatus) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Status = status
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentRepoData(ctx context.Context, name string, rrd *RepoRevisionData) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Repo = rrd.Repo
		v.PullRequest = rrd.PullRequest
		v.BaseSHA = rrd.BaseSHA
		v.BaseBranch = rrd.BaseBranch
		v.SourceRef = rrd.SourceRef
		v.SourceSHA = rrd.SourceSHA
		v.SourceBranch = rrd.SourceBranch
		v.User = rrd.User
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentRefMap(ctx context.Context, name string, rm RefMap) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.RefMap = rm
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentCommitSHAMap(ctx context.Context, name string, csm RefMap) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.CommitSHAMap = csm
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetQAEnvironmentCreated(ctx context.Context, name string, ts time.Time) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.Created = ts
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoEnvironmentID(ctx context.Context, name string, did int) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoEnvironmentID = did
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoServiceToPort(ctx context.Context, name string, serviceToPort map[string]int64) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoServiceToPort = serviceToPort
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) SetAminoKubernetesNamespace(ctx context.Context, name, namespace string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		v.AminoKubernetesNamespace = namespace
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) AddEvent(ctx context.Context, name string, msg string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.d[name]; ok {
		e := models.QAEnvironmentEvent{
			Timestamp: time.Now().UTC(),
			Message:   msg,
		}
		v.Events = append(v.Events, e)
		return nil
	}
	return errors.New("env not found")
}

func (fdl *FakeDataLayer) getQAEnvironmentsBySourceRef(sourceref string) ([]QAEnvironment, error) {
	out := []models.QAEnvironment{}
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	for _, v := range fdl.data.d {
		if v.SourceRef == sourceref {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) Search(ctx context.Context, opts models.EnvSearchParameters) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	if opts.Pr != 0 && opts.Repo == "" {
		return nil, fmt.Errorf("search by PR requires repo name")
	}
	if opts.TrackingRef != "" && opts.Repo == "" {
		return nil, fmt.Errorf("search by tracking ref requires repo name")
	}
	filter := func(envs []models.QAEnvironment, cf func(e models.QAEnvironment) bool) []models.QAEnvironment {
		pres := []models.QAEnvironment{}
		for _, e := range envs {
			if cf(e) {
				pres = append(pres, e)
			}
		}
		return pres
	}
	envs, _ := fdl.GetQAEnvironments(ctx)
	if opts.Pr != 0 {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.PullRequest == opts.Pr && e.Repo == opts.Repo })
	}
	if opts.Repo != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.Repo == opts.Repo })
	}
	if opts.SourceSHA != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceSHA == opts.SourceSHA })
	}
	if opts.SourceBranch != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceBranch == opts.SourceBranch })
	}
	if opts.User != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.User == opts.User })
	}
	if opts.Status != models.UnknownStatus {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.Status == opts.Status })
	}
	if opts.TrackingRef != "" {
		envs = filter(envs, func(e models.QAEnvironment) bool { return e.SourceRef == opts.TrackingRef })
	}
	return envs, nil
}

func (fdl *FakeDataLayer) GetMostRecent(ctx context.Context, n uint) ([]QAEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	envs, _ := fdl.GetQAEnvironments(ctx)
	sort.Slice(envs, func(i int, j int) bool { return envs[i].Created.After(envs[j].Created) })
	if int(n) > len(envs) {
		return envs, nil
	}
	return envs[0:n], nil
}

func (fdl *FakeDataLayer) Close() error {
	return nil
}

func (fdl *FakeDataLayer) GetHelmReleasesForEnv(ctx context.Context, name string) ([]models.HelmRelease, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	v, ok := fdl.data.helm[name]
	if ok {
		return v, nil
	}
	return nil, nil
}

func (fdl *FakeDataLayer) UpdateHelmReleaseRevision(ctx context.Context, envname, release, revision string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	for i, r := range fdl.data.helm[envname] {
		if r.Release == release {
			fdl.data.helm[envname][i].RevisionSHA = revision
		}
	}
	return nil
}

func (fdl *FakeDataLayer) CreateHelmReleasesForEnv(ctx context.Context, releases []models.HelmRelease) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	if len(releases) == 0 {
		return errors.New("empty releases")
	}
	name := releases[0].EnvName
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if _, ok := fdl.data.d[name]; !ok {
		return errors.New("env missing")
	}
	fdl.data.helm[name] = releases
	return nil
}

func (fdl *FakeDataLayer) DeleteHelmReleasesForEnv(ctx context.Context, name string) (uint, error) {
	if isCancelled(ctx) {
		return 0, ctx.Err()
	}
	fdl.doDelay()
	var n uint
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if v, ok := fdl.data.helm[name]; ok {
		n = uint(len(v))
	} else {
		return 0, nil
	}
	delete(fdl.data.helm, name)
	return n, nil
}

func (fdl *FakeDataLayer) GetK8sEnv(ctx context.Context, name string) (*models.KubernetesEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	env, ok := fdl.data.k8s[name]
	if !ok {
		return nil, nil
	}
	return env, nil
}

func (fdl *FakeDataLayer) GetK8sEnvsByNamespace(ctx context.Context, ns string) ([]models.KubernetesEnvironment, error) {
	if isCancelled(ctx) {
		return nil, ctx.Err()
	}
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.KubernetesEnvironment
	for _, v := range fdl.data.k8s {
		if v.Namespace == ns {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) CreateK8sEnv(ctx context.Context, env *models.KubernetesEnvironment) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if env == nil || env.EnvName == "" {
		return errors.New("malformed env: nil or empty name")
	}
	if _, ok := fdl.data.d[env.EnvName]; !ok {
		return errors.New("env not found")
	}
	fdl.data.k8s[env.EnvName] = env
	return nil
}

func (fdl *FakeDataLayer) DeleteK8sEnv(ctx context.Context, name string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	delete(fdl.data.k8s, name)
	return nil
}

func (fdl *FakeDataLayer) UpdateK8sEnvTillerAddr(ctx context.Context, envname, taddr string) error {
	if isCancelled(ctx) {
		return ctx.Err()
	}
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	env, ok := fdl.data.k8s[envname]
	if ok {
		env.TillerAddr = taddr
		fdl.data.k8s[envname] = env
	}
	return nil
}

func (fdl *FakeDataLayer) GetEventLogByID(id uuid.UUID) (*models.EventLog, error) {
	fdl.doDelay()
	fdl.data.RLock()
	el, ok := fdl.data.elogs[id]
	fdl.data.RUnlock()
	if !ok {
		if fdl.CreateMissingEventLog {
			sum := fdl.newStatus(id)
			out := &models.EventLog{
				ID:     id,
				LogKey: uuid.Must(uuid.NewRandom()),
				Status: *sum,
			}
			fdl.data.Lock()
			fdl.data.elogs[id] = out
			fdl.data.Unlock()
			return out, nil
		}
		return nil, nil
	}
	return el, nil
}

func (fdl *FakeDataLayer) GetEventLogByDeliveryID(deliveryID uuid.UUID) (*models.EventLog, error) {
	fdl.doDelay()
	fdl.data.RLock()
	var el models.EventLog
	for _, elog := range fdl.data.elogs {
		if elog.GitHubDeliveryID == deliveryID {
			el = *elog
		}
	}
	fdl.data.RUnlock()
	return &el, nil
}

func (fdl *FakeDataLayer) GetEventLogsByEnvName(name string) ([]models.EventLog, error) {
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.EventLog
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].EnvName == name {
			out = append(out, *fdl.data.elogs[k])
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) GetEventLogsByRepoAndPR(repo string, pr uint) ([]models.EventLog, error) {
	fdl.doDelay()
	fdl.data.RLock()
	defer fdl.data.RUnlock()
	var out []models.EventLog
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].Repo == repo && fdl.data.elogs[k].PullRequest == pr {
			out = append(out, *fdl.data.elogs[k])
		}
	}
	return out, nil
}

func (fdl *FakeDataLayer) CreateEventLog(elog *models.EventLog) error {
	fdl.doDelay()
	if elog == nil {
		return errors.New("input is nil")
	}
	if elog.LogKey == uuid.Nil {
		lk, err := uuid.NewRandom()
		if err != nil {
			return errors.Wrap(err, "error generating log key")
		}
		elog.LogKey = lk
	}
	fdl.data.Lock()
	defer fdl.data.Unlock()
	fdl.data.elogs[elog.ID] = elog
	return nil
}

func (fdl *FakeDataLayer) AppendToEventLog(id uuid.UUID, msg string) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if fdl.data.elogs[id] == nil {
		return errors.New("id not found")
	}
	fdl.data.elogs[id].Log = append(fdl.data.elogs[id].Log, msg)
	return nil
}

func (fdl *FakeDataLayer) SetEventLogEnvName(id uuid.UUID, name string) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog != nil {
		fdl.data.elogs[id].EnvName = name
	}
	return nil
}

func (fdl *FakeDataLayer) DeleteEventLog(id uuid.UUID) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	delete(fdl.data.elogs, id)
	return nil
}

func (fdl *FakeDataLayer) DeleteEventLogsByEnvName(name string) (uint, error) {
	fdl.doDelay()
	del := []uuid.UUID{}
	fdl.data.Lock()
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].EnvName == name {
			del = append(del, fdl.data.elogs[k].ID)
		}
	}
	for _, id := range del {
		delete(fdl.data.elogs, id)
	}
	fdl.data.Unlock()
	return uint(len(del)), nil
}

func (fdl *FakeDataLayer) DeleteEventLogsByRepoAndPR(repo string, pr uint) (uint, error) {
	fdl.doDelay()
	del := []uuid.UUID{}
	fdl.data.Lock()
	for k := range fdl.data.elogs {
		if fdl.data.elogs[k].Repo == repo && fdl.data.elogs[k].PullRequest == pr {
			del = append(del, fdl.data.elogs[k].ID)
		}
	}
	for _, id := range del {
		delete(fdl.data.elogs, id)
	}
	fdl.data.Unlock()
	return uint(len(del)), nil
}

func (fdl *FakeDataLayer) SetEventStatus(id uuid.UUID, status models.EventStatusSummary) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	elog.Status = status
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusConfig(id uuid.UUID, processingTime time.Duration, refmap map[string]string) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	elog.Status.Config.ProcessingTime = models.ConfigProcessingDuration{Duration: processingTime}
	elog.Status.Config.RefMap = refmap
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusConfigK8sNS(id uuid.UUID, ns string) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	elog.Status.Config.K8sNamespace = ns
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusTree(id uuid.UUID, tree map[string]models.EventStatusTreeNode) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	elog.Status.Tree = tree
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusCompleted(id uuid.UUID, status models.EventStatus) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	elog.Status.Config.Status = status
	elog.Status.Config.Completed = time.Now().UTC()
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusImageStarted(id uuid.UUID, name string) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	tn, ok := elog.Status.Tree[name]
	if !ok {
		keys := make([]string, len(elog.Status.Tree))
		i := 0
		for k := range elog.Status.Tree {
			keys[i] = k
			i++
		}
		return fmt.Errorf("%v not found in tree: %v: %v", name, len(keys), keys)
	}
	tn.Image.Started = time.Now().UTC()
	fdl.data.elogs[id].Status.Tree[name] = tn
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusImageCompleted(id uuid.UUID, name string, err bool) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	tn, ok := elog.Status.Tree[name]
	if !ok {
		return fmt.Errorf("%v not found in tree", name)
	}
	tn.Image.Error = err
	tn.Image.Completed = time.Now().UTC()
	elog.Status.Tree[name] = tn
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusChartStarted(id uuid.UUID, name string, status models.NodeChartStatus) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	tn, ok := elog.Status.Tree[name]
	if !ok {
		return fmt.Errorf("%v not found in tree", name)
	}
	tn.Chart.Status = status
	tn.Chart.Started = time.Now().UTC()
	elog.Status.Tree[name] = tn
	return nil
}

func (fdl *FakeDataLayer) SetEventStatusChartCompleted(id uuid.UUID, name string, status models.NodeChartStatus) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog == nil {
		return errors.New("eventlog not found")
	}
	tn, ok := elog.Status.Tree[name]
	if !ok {
		return fmt.Errorf("%v not found in tree", name)
	}
	tn.Chart.Status = status
	tn.Chart.Completed = time.Now().UTC()
	elog.Status.Tree[name] = tn
	return nil
}

func (fdl *FakeDataLayer) GetEventStatus(id uuid.UUID) (*models.EventStatusSummary, error) {
	fdl.doDelay()
	fdl.data.RLock()
	elog := fdl.data.elogs[id]
	fdl.data.RUnlock()
	if elog == nil {
		if fdl.CreateMissingEventLog {
			// if id not found, create a new one and begin an async update goroutine
			return fdl.newStatus(id), nil
		}
		return nil, nil
	}
	out := *elog
	return &out.Status, nil
}

func (fdl *FakeDataLayer) SetEventStatusRenderedStatus(id uuid.UUID, rstatus models.RenderedEventStatus) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	elog := fdl.data.elogs[id]
	if elog != nil {
		elog.Status.Config.RenderedStatus = rstatus
	}
	return nil
}

func (fdl *FakeDataLayer) CreateUISession(key, data, state []byte, expires time.Time) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if len(key) == 0 || len(data) == 0 || len(state) == 0 {
		return errors.New("key, data and state are required")
	}
	if expires.IsZero() || time.Now().UTC().After(expires) {
		return fmt.Errorf("invalid expires time: %v", expires)
	}
	fdl.data.uisessions[string(key)] = &models.UISession{
		Key:     key,
		Data:    data,
		State:   state,
		Expires: expires,
	}
	return nil
}

func (fdl *FakeDataLayer) UpdateUISession(key, data []byte, expires time.Time) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if len(key) == 0 || len(data) == 0 {
		return errors.New("key and data are required")
	}
	if expires.IsZero() || time.Now().UTC().After(expires) {
		return fmt.Errorf("invalid expires time: %v", expires)
	}
	uis, ok := fdl.data.uisessions[string(key)]
	if !ok {
		return nil
	}
	uis.Data = data
	uis.Expires = expires
	return nil
}

func (fdl *FakeDataLayer) DeleteUISession(key []byte) error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	delete(fdl.data.uisessions, string(key))
	return nil
}

func (fdl *FakeDataLayer) GetUISession(key []byte) (*models.UISession, error) {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	if len(key) == 0 {
		return nil, errors.New("key is required")
	}
	uis, ok := fdl.data.uisessions[string(key)]
	if !ok {
		return nil, nil
	}
	out := *uis
	return &out, nil
}

func (fdl *FakeDataLayer) DeleteExpiredUISessions() error {
	fdl.doDelay()
	fdl.data.Lock()
	defer fdl.data.Unlock()
	rmkeys := []string{}
	for keystr, val := range fdl.data.uisessions {
		if val.Expires.Before(time.Now().UTC()) {
			rmkeys = append(rmkeys, keystr)
		}
	}
	for _, kstr := range rmkeys {
		delete(fdl.data.uisessions, kstr)
	}
	return nil
}
