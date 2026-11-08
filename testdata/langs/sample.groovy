package com.example

import groovy.json.JsonSlurper
import java.util.List

interface Greeter {
    String greet(String name)
}

class Person implements Greeter {
    String name
    String greet(String who) { "hi $who" }
    private void secret() {}
}

def helper(int a, int b) {
    a + b
}

enum Level { LOW, HIGH }
