package router

import "github.com/julienschmidt/httprouter"

type RouterGroup struct {
	router *httprouter.Router
	prefix string
}

func New(router *httprouter.Router) *RouterGroup {
	return &RouterGroup{
		router: router,
	}
}

func (rg *RouterGroup) Group(prefix string) *RouterGroup {
	rg.prefix = prefix
	return rg
}

func (rg *RouterGroup) GET(path string, handle httprouter.Handle) {
	rg.router.GET(rg.prefix+path, handle)
}

func (rg *RouterGroup) POST(path string, handle httprouter.Handle) {
	rg.router.POST(rg.prefix+path, handle)
}

func (rg *RouterGroup) PUT(path string, handle httprouter.Handle) {
	rg.router.PUT(rg.prefix+path, handle)
}

func (rg *RouterGroup) DELETE(path string, handle httprouter.Handle) {
	rg.router.DELETE(rg.prefix+path, handle)
}

func (rg *RouterGroup) PATCH(path string, handle httprouter.Handle) {
	rg.router.PATCH(rg.prefix+path, handle)
}

func (rg *RouterGroup) HEAD(path string, handle httprouter.Handle) {
	rg.router.HEAD(rg.prefix+path, handle)
}

func (rg *RouterGroup) OPTIONS(path string, handle httprouter.Handle) {
	rg.router.OPTIONS(rg.prefix+path, handle)
}
