package main

import (
	"fmt"
	"net"

	"github.com/golang/glog"
)

type SessionManager struct {
	config            *Config                      // Configure
	tcpListener       net.Listener                 // TCP listening object
	sessionIDManager  *SessionIDManager            // Session ID Manager
	upSessionManagers map[string]*UpSessionManager // MAP [Sub Account Name] Mining Session Manager
	exitChannel       chan bool                    //Exit signal
	eventChannel      chan interface{}             // Event cycle
}

func NewSessionManager(config *Config) (manager *SessionManager) {
	manager = new(SessionManager)
	manager.config = config
	manager.upSessionManagers = make(map[string]*UpSessionManager)
	manager.exitChannel = make(chan bool, 1)
	manager.eventChannel = make(chan interface{}, manager.config.Advanced.MessageQueueSize.SessionManager)
	return
}

func (manager *SessionManager) Run() {
	var err error

	// Initialization Session Manager
	manager.sessionIDManager, err = NewSessionIDManager(0xfffe)
	if err != nil {
		glog.Fatal("NewSessionIDManager failed: ", err)
		return
	}

	// Start event loop
	go manager.handleEvent()

	// TCP listening
	listenAddr := fmt.Sprintf("%s:%d", manager.config.AgentListenIp, manager.config.AgentListenPort)
	glog.Info("startup is successful, listening: ", listenAddr)
	manager.tcpListener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		glog.Fatal("failed to listen on ", listenAddr, ": ", err)
		return
	}

	// Connect the mine for single user mode
	if !manager.config.MultiUserMode {
		manager.createUpSessionManager("")
	}

	for {
		conn, err := manager.tcpListener.Accept()
		if err != nil {
			select {
			case <-manager.exitChannel:
				return
			default:
				glog.Warning("failed to accept miner connection: ", err.Error())
				continue
			}
		}
		go manager.RunDownSession(conn)
	}
}

func (manager *SessionManager) Stop() {
	// Exit TCP listening
	manager.exitChannel <- true
	manager.tcpListener.Close()

	// Exit the event cycle
	manager.SendEvent(EventExit{})
}

func (manager *SessionManager) exit() {
	// Require all connection to exit
	for _, up := range manager.upSessionManagers {
		up.SendEvent(EventExit{})
	}
}

func (manager *SessionManager) RunDownSession(conn net.Conn) {
	// produce sessionID （Extranonce1）
	sessionID, err := manager.sessionIDManager.AllocSessionID()

	if err != nil {
		glog.Warning("failed to allocate session id : ", err)
		conn.Close()
		return
	}

	down := manager.config.sessionFactory.NewDownSession(manager, conn, sessionID)
	down.Init()
	if down.Stat() != StatAuthorized {
		// Certification failed, abandon connection
		return
	}

	go down.Run()

	manager.SendEvent(EventAddDownSession{down})
}

func (manager *SessionManager) SendEvent(event interface{}) {
	manager.eventChannel <- event
}

func (manager *SessionManager) createUpSessionManager(subAccount string) (upManager *UpSessionManager) {
	upManager = NewUpSessionManager(subAccount, manager.config, manager)
	go upManager.Run()
	manager.upSessionManagers[subAccount] = upManager
	return
}

func (manager *SessionManager) addDownSession(e EventAddDownSession) {
	upManager, ok := manager.upSessionManagers[e.Session.SubAccountName()]
	if !ok {
		upManager = manager.createUpSessionManager(e.Session.SubAccountName())
	}
	upManager.SendEvent(e)
}

func (manager *SessionManager) stopUpSessionManager(e EventStopUpSessionManager) {
	child := manager.upSessionManagers[e.SubAccount]
	if child == nil {
		glog.Error("StopUpSessionManager: cannot find sub-account: ", e.SubAccount)
		return
	}
	delete(manager.upSessionManagers, e.SubAccount)
	child.SendEvent(EventExit{})
}

func (manager *SessionManager) handleEvent() {
	for {
		event := <-manager.eventChannel

		switch e := event.(type) {
		case EventAddDownSession:
			manager.addDownSession(e)
		case EventStopUpSessionManager:
			manager.stopUpSessionManager(e)
		case EventExit:
			manager.exit()
			return
		default:
			glog.Error("[SessionManager] unknown event: ", event)
		}
	}
}
