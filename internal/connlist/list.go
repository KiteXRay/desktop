package connlist

import (
	"errors"
	"slices"
	"sync"
)

// Collection represents a collection of items.
// Is used to easily pass events and update the UI state in one place (on{*} methods).
type Collection struct {
	items []*Item
	mu    sync.RWMutex

	onAdd    func(*Item)
	onDelete func(*Item)
	onSwap   func(*Item, *Item)
	onChange func()
}

func New() *Collection {
	items := &Collection{items: make([]*Item, 0)}
	items.OnAdd(func(item *Item) {})
	items.OnDelete(func(item *Item) {})
	items.OnSwap(func(i1, i2 *Item) {})
	items.OnChange(func() {})

	return items
}

func (l *Collection) AllUntyped() *[]any {
	all := l.All()
	bindItems := make([]any, len(all))
	for i, item := range all {
		bindItems[i] = item
	}

	return &bindItems
}

func (l *Collection) OnAdd(onAdd func(item *Item)) {
	l.onAdd = func(i *Item) {
		onAdd(i)
		l.onChange()
	}
}

func (l *Collection) OnSwap(onSwap func(*Item, *Item)) {
	l.onSwap = func(i1 *Item, i2 *Item) {
		onSwap(i1, i2)
		l.onChange()
	}
}

// OnDelete note: provided method is called before the actual deletion of the item.
func (l *Collection) OnDelete(onDelete func(item *Item)) {
	l.onDelete = onDelete
}

func (l *Collection) OnChange(onChange func()) {
	l.onChange = onChange
}

func (l *Collection) All() []*Item {
	l.mu.RLock()
	defer l.mu.RUnlock()

	res := make([]*Item, 0, len(l.items))
	for _, item := range l.items {
		if item == nil {
			continue
		}
		res = append(res, item)
	}

	return res
}

func (l *Collection) FindByID(id string) *Item {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, item := range l.items {
		if item != nil && item.ID() == id {
			return item
		}
	}
	return nil
}

func (l *Collection) AddItem(label, link string) error {
	return l.AddItemWithTraffic(label, link, 0, 0)
}

func (l *Collection) AddItemWithTraffic(label, link string, read, written int64) error {
	return l.AddItemWithID("", label, link, read, written)
}

func (l *Collection) AddItemWithID(id, label, link string, read, written int64) error {
	item, err := newItemWithID(id, label, link, l)
	if err != nil {
		return err
	}
	item.SetPersistedTraffic(read, written)

	l.mu.Lock()
	l.items = append(l.items, item)
	onAdd := l.onAdd
	l.mu.Unlock()

	if onAdd != nil {
		onAdd(item)
	}

	return nil
}

func (l *Collection) RemoveItem(del *Item) {
	if del == nil {
		return
	}

	l.mu.Lock()
	idx := -1
	for i, item := range l.items {
		if item == del {
			idx = i
			break
		}
	}
	if idx == -1 {
		l.mu.Unlock()
		return
	}

	item := l.items[idx]
	l.items = slices.Delete(l.items, idx, idx+1)
	onDelete := l.onDelete
	onChange := l.onChange
	l.mu.Unlock()

	if onDelete != nil {
		onDelete(item)
	}
	if item != nil {
		item.Close()
	}
	if onChange != nil {
		onChange()
	}
}

func (l *Collection) SwapItems(itm1 *Item, itm2 *Item) error {
	l.mu.Lock()
	id1, id2 := -1, -1
	for i, item := range l.items {
		if item == itm1 {
			id1 = i
		}
		if item == itm2 {
			id2 = i
		}
	}
	if id1 == -1 || id2 == -1 {
		l.mu.Unlock()
		return errors.New("cannot swap items")
	}

	l.items[id1], l.items[id2] = l.items[id2], l.items[id1]
	onSwap := l.onSwap
	onChange := l.onChange
	l.mu.Unlock()

	if onSwap != nil {
		onSwap(itm1, itm2)
	} else if onChange != nil {
		onChange()
	}

	return nil
}

func (l *Collection) MoveItem(from, to int) error {
	l.mu.Lock()
	if from < 0 || from >= len(l.items) || to < 0 || to >= len(l.items) {
		l.mu.Unlock()
		return errors.New("index out of bounds")
	}
	if from == to {
		l.mu.Unlock()
		return nil
	}

	item := l.items[from]
	l.items = slices.Delete(l.items, from, from+1)
	l.items = slices.Insert(l.items, to, item)
	onChange := l.onChange
	l.mu.Unlock()

	if onChange != nil {
		onChange()
	}

	return nil
}
