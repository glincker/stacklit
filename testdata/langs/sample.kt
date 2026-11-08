package com.example.app

import kotlin.collections.List
import com.example.util.*
import com.example.model.User as AppUser

interface Repository {
    fun find(id: Int): User?
}

data class User(val id: Int, val name: String)

open class Service(private val repo: Repository) {
    val label: String = "svc"
    fun load(id: Int): User? = repo.find(id)
    private fun hidden() {}
}

object Registry {
    fun register(name: String) {}
}

enum class Level { LOW, HIGH }

private class Internal

fun topLevel(a: Int, b: String): Boolean {
    return true
}

private fun secret() {}
