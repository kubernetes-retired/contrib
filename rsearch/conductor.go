package rsearch

func manageResources(ns NsEvent, terminators map[string]chan Done, config Config, out chan Event) {
	uid := ns.Object.Metadata.Uid
	if ns.Type == "ADDED" {
		done := make(chan Done)
		terminators[uid] = done
		ns.Object.Produce(out, terminators[uid], config)
	} else if ns.Type == "DELETED" {
		close(terminators[uid])
		delete(terminators, uid)
	} else if ns.Type == "_CRASH" {
		for uid, c := range terminators {
			close(c)
			delete(terminators, uid)
		}
	}
}

// Conductor manages a set of goroutines one per namespace.
func Conductor(in <-chan NsEvent, done <-chan Done, config Config) <-chan Event {
	// Idea of this map is to keep termination channels organized
	// so when DELETED event occurs on a namespace it would be possible
	// to terminater related goroutine
	var terminators map[string]chan Done
	terminators = make(map[string]chan Done)

	ns := NsEvent{}
	out := make(chan Event)

	go func() {
		for {
			select {
			case ns = <-in:
				manageResources(ns, terminators, config, out)
			case <-done:
				return
			}
		}
	}()

	return out
}
