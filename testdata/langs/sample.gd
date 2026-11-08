class_name Player
extends CharacterBody2D

signal died(reason)

const SPEED = 300.0
var health: int = 100
@export var max_health: int = 100

func _ready() -> void:
	pass

func take_damage(amount: int) -> void:
	health -= amount

func _private_like() -> int:
	return 1

class Inner:
	var x = 1
	func foo():
		pass

enum State { IDLE, RUN }
