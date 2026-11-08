import Foundation

class App: Base {
    var repo: Base

    func run(b: Base) {
        helper(1)
        greet()
        b.greet()
        repo.greet()
    }
}
