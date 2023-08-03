package drugdose

import (
	"sync"
)

type ChannelStructs interface {
	UserLogsError | DrugNamesError | DrugInfoError |
		UserSettingError | LogCountError | AllUsersError | ErrorInfo
}

// AddChannelHandler starts receiving from a channel which it creates, using
// the structure given as the first argument of the handler function.
// The structure indicates from which goroutine it will receive data.
// For example the structure UserLogsError is related to the function GetLogs()
// since it is accepted as an argument by that function. This function will
// return the channel needed to be passed to GetLogs() if UserLogsError is used.
//
// If the handler function returns false, the handler loop is stopped,
// the wait group counter is decremented by one and the channel won't accept
// any more data. It would be good to make sure that all sending goroutines
// like GetLogs() are stopped beforehand.
//
// Using AddChannelHandler() isn't strictly required, creating channels
// manually and receiving data from them is acceptable as well. This function
// should be used when needed, for example when accepting multiple requests
// at once.
//
// wg - the wait group has to be initialized manually and at the end of the
// main() function, wg.Wait() has to be executed, replacing wg with the name
// given when initializing the wait group
//
// handler - the handler function has to be created manually using the type
// information defined in this function, it will be executed every time data
// is received from the running goroutines
func AddChannelHandler[CS ChannelStructs](wg *sync.WaitGroup,
	handler func(cs CS, a ...any) bool, a ...any) chan CS {

	newChannel := make(chan CS)

	wg.Add(1)
	go func() {
		var gotNewChan CS
		for {
			gotNewChan = <-newChannel
			ret := handler(gotNewChan, a...)
			if ret == false {
				break
			}
		}
		wg.Done()
	}()
	return newChannel
}
