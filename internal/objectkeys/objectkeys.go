// Package objectkeys da las claves de un objeto JSON decodificado en orden de nombre, para que
// los issues de una entrada salgan siempre en el mismo orden y no en el aleatorio de los mapas de Go
// (#174).
package objectkeys

import (
	"slices"
	"sync"
)

// pool presta los buffers de claves: ordenar las claves de cada objeto de cada recurso no asigna
// memoria por objeto.
var pool = sync.Pool{New: func() any { return new([]string) }}

// Keys son las claves de un objeto, en orden de nombre, en un buffer prestado. Se devuelve con
// Release; después, List no se usa más.
type Keys struct {
	List []string
	buf  *[]string
}

// Sorted devuelve las claves de m ordenadas por nombre.
func Sorted[V any](m map[string]V) Keys {
	buf, _ := pool.Get().(*[]string)
	list := (*buf)[:0]
	for k := range m {
		list = append(list, k)
	}
	slices.Sort(list)
	*buf = list
	return Keys{List: list, buf: buf}
}

// Release devuelve el buffer. Sus strings se borran para no retener el documento validado.
func (k Keys) Release() {
	clear(*k.buf)
	*k.buf = (*k.buf)[:0]
	pool.Put(k.buf)
}
