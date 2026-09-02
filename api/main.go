package main

func main() {
	router := chi.NewMux()
	api := humachi.New(router, huma.DefaultConfig("My API", "1.0.0"))
	
	http.ListenAndServe("127.0.0.1:8888", router)
}