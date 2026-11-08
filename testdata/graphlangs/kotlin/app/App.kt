package app

import lib.Base
import lib.helper

class App(val repo: Base) : Base() {
    fun run(b: Base) {
        helper(1)
        greet()
        b.greet()
        repo.greet()
    }
}
