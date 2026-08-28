package we_work

import (
	"core/common/log"
	"core/pkg"
	"core/pkg/easywework"
	"github.com/pkg/errors"
	"sync"
)

var App *workwx.App

var (
	Clients *WeWorkClients
	once    sync.Once
)

type CorpConf struct {
	AuthKey string `json:"-"`
	// ExtCorpID 外部企业ID
	ExtCorpID string `json:"ext_corp_id" validate:"ext_corp_id" gorm:"unique;type:char(18);comment:外部企业ID"`
	// ContactSecret 通讯录secret
	ContactSecret string `json:"contact_secret" gorm:"comment:'通讯录secret'"`
	// CustomerSecret 客户联系secret
	CustomerSecret string `json:"customer_secret" gorm:"comment:'客户联系secret'"`
	// MainAgentID 企业主应用AgentID
	MainAgentID int64 `json:"main_agent_id" gorm:"comment:'企业主应用AgentID'" validate:"number,gte=0"`
	// MainAgentSecret 企业主应用secret
	MainAgentSecret string `json:"main_agent_secret" gorm:"comment:'企业主应用secret'"`
}

func (clients *WeWorkClients) AddAuthorized(conf CorpConf, fetch func() (string, int64, error)) error {
	clients.mutex.Lock()
	defer clients.mutex.Unlock()
	if existing, ok := clients.clients[conf.ExtCorpID]; ok && existing.config == conf {
		return nil
	}
	app, err := NewWxApp(conf.ExtCorpID, "", conf.MainAgentID)
	if err != nil {
		return err
	}
	app.UseAccessTokenProvider(fetch)
	clients.clients[conf.ExtCorpID] = Client{config: conf, MainApp: app, Contact: app, Customer: app}
	return nil
}

type Client struct {
	config   CorpConf
	MainApp  *workwx.App
	Contact  *workwx.App
	Customer *workwx.App
}

type WeWorkClients struct {
	mutex sync.Mutex
	//todo concurrent.Map
	//clients map[string]*gowx.WorkwxApp
	clients map[string]Client
}

// SetupClient 在连接db后init
func SetupClient(conf CorpConf) {
	EnsureClients()
	if err := Clients.Add(conf); err != nil {
		log.Sugar.Errorw("NewClient failed", "err", err)
	}
}

// EnsureClients 懒初始化全局 Clients map（多租户 per-corp 运行时首次注册前调用）。
// SetupClient 和多租户渠道注册共用同一初始化入口，后续按 corp 追加客户端。
func EnsureClients() {
	once.Do(func() { Clients = &WeWorkClients{mutex: sync.Mutex{}, clients: map[string]Client{}} })
}

func NewClient(conf CorpConf) (client Client, err error) {
	client.config = conf
	client.Contact, err = NewWxApp(conf.ExtCorpID, conf.ContactSecret, 0)
	if err != nil {
		err = errors.Wrap(err, "new Contact app failed")
		return
	}

	client.Customer, err = NewWxApp(conf.ExtCorpID, conf.CustomerSecret, 0)
	if err != nil {
		err = errors.Wrap(err, "new Customer app failed")
		return
	}

	client.MainApp, err = NewWxApp(conf.ExtCorpID, conf.MainAgentSecret, conf.MainAgentID)
	if err != nil {
		err = errors.Wrap(err, "new main app failed")
		return
	}

	return
}

func (clients *WeWorkClients) Get(extCorpID string) (client Client, err error) {
	if clients == nil {
		return client, errors.New("WeCom clients are not configured")
	}
	var ok bool
	clients.mutex.Lock()
	defer clients.mutex.Unlock()
	client, ok = clients.clients[extCorpID]
	if !ok {
		err = errors.New("invalid extCorpID")
		return
	}

	return
}

func (clients *WeWorkClients) Remove(extCorpID string) {
	clients.mutex.Lock()
	defer clients.mutex.Unlock()

	delete(clients.clients, extCorpID)
}

func (clients *WeWorkClients) Add(conf CorpConf) (err error) {
	clients.mutex.Lock()
	defer clients.mutex.Unlock()
	if existing, ok := clients.clients[conf.ExtCorpID]; ok && existing.config == conf {
		return nil
	}
	client, err := NewClient(conf)
	if err != nil {
		err = errors.Wrap(err, "NewClient failed")
		return
	}
	clients.clients[conf.ExtCorpID] = client

	return
}

func NewWxApp(extCorpID string, secret string, agentID int64) (wxApp *workwx.App, err error) {
	cliOptions := pkg.CliOptions{
		CorpID:            extCorpID,
		CorpSecret:        secret,
		AgentID:           agentID,
		QYAPIHostOverride: "",
		TLSKeyLogFile:     "",
	}

	wxApp = cliOptions.MakeWorkwxApp().WithApp(cliOptions.CorpSecret, cliOptions.AgentID)
	//_, err = wxApp.GetToken()
	//if err != nil {
	//	err = errors.Wrap(err, "get token failed")
	//	return
	//}

	return
}
