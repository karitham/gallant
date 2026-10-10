package sample

// User is a user.
type User struct {
	// Name is the name.
	Name string
	// Age is the age.
	Age int
}

// Add returns a plus b.
func Add(a, b int) int {
	return a + b // return a plus b
}

// Run creates a user and prints their name.
func Run() {
	// create a new user
	u := User{Name: "Ada", Age: 36}
	// print the name
	println(u.Name) // print the name
	// increment the age
	u.Age = u.Age + 1 // add one to age
}
