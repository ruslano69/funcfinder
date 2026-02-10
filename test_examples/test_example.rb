# Ruby test file for funcfinder

require 'json'

# Simple function
def greet(name)
  "Hello, #{name}!"
end

# Function with default parameter
def add(a, b = 0)
  a + b
end

# Function with keyword arguments
def create_person(name:, age: 0)
  { name: name, age: age }
end

# Function with block
def with_logging(&block)
  puts "[START]"
  result = block.call
  puts "[END]"
  result
end

# Class
class Person
  attr_accessor :name, :age

  def initialize(name, age = 0)
    @name = name
    @age = age
  end

  def describe
    "#{@name}, age #{@age}"
  end

  def self.create(name)
    new(name)
  end

  private

  def internal_method
    # private method
  end
end

# Module
module Loggable
  def log(message)
    puts "[LOG] #{message}"
  end

  def error(message)
    puts "[ERROR] #{message}"
  end

  module_function

  def module_level_log(message)
    puts "[MODULE] #{message}"
  end
end

# Class with module include
class Logger
  include Loggable

  def initialize(prefix)
    @prefix = prefix
  end

  def info(message)
    log("#{@prefix}: #{message}")
  end
end

# Struct
Point = Struct.new(:x, :y) do
  def distance(other)
    Math.sqrt((x - other.x)**2 + (y - other.y)**2)
  end

  def to_s
    "(#{x}, #{y})"
  end
end

# Class inheritance
class Shape
  def area
    raise NotImplementedError
  end

  def perimeter
    raise NotImplementedError
  end

  def describe
    "Area: #{area}, Perimeter: #{perimeter}"
  end
end

class Circle < Shape
  attr_reader :radius

  def initialize(radius)
    @radius = radius
  end

  def area
    Math::PI * @radius**2
  end

  def perimeter
    2 * Math::PI * @radius
  end
end

class Rectangle < Shape
  attr_reader :width, :height

  def initialize(width, height)
    @width = width
    @height = height
  end

  def area
    @width * @height
  end

  def perimeter
    2 * (@width + @height)
  end
end

# Singleton class
class Config
  @instance = nil

  private_class_method :new

  def self.instance
    @instance ||= new
  end

  def get(key)
    # get config value
  end

  def set(key, value)
    # set config value
  end
end

# Module with class methods
module Calculator
  class << self
    def add(a, b)
      a + b
    end

    def subtract(a, b)
      a - b
    end

    def multiply(a, b)
      a * b
    end
  end
end

# Lambda (should not be detected as function)
multiply = ->(a, b) { a * b }
square = lambda { |x| x * x }

# Proc
double = Proc.new { |x| x * 2 }
