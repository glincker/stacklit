import 'dart:async';
import 'package:flutter/material.dart';
import 'src/utils.dart' as utils;
export 'src/models.dart';

part 'sample.g.dart';

abstract class Shape {
  double area();
}

class Circle extends Shape with Printable implements Comparable<Circle> {
  final double radius;
  Circle(this.radius);

  @override
  double area() => 3.14 * radius * radius;

  Future<void> save(String path, {bool force = false}) async {}

  String _secret() => 'x';
}

mixin Printable {
  void printMe() {}
}

enum Status { active, inactive }

extension StringExt on String {
  String shout() => toUpperCase();
}

typedef Callback = void Function(int);

String greet(String name) {
  return 'Hello $name';
}

int _private(int x) => x;

void main() {
  runApp(const MyApp());
}
