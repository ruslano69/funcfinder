# Python test file for findstruct prototype
# Tests: classes, dataclass, NamedTuple, TypedDict, Enum, attrs

from dataclasses import dataclass
from typing import NamedTuple, TypedDict, List, Optional
from enum import Enum
import attr


# Simple class
class Point:
    x: int
    y: int
    z: float = 0.0

    def __init__(self, x, y):
        self.x = x
        self.y = y

    def distance_to(self, other: 'Point') -> float:
        return ((self.x - other.x) ** 2 + (self.y - other.y) ** 2) ** 0.5


# Class with various field types
class User:
    id: int
    name: str
    email: str
    active: bool = True
    roles: List[str] = None

    def __init__(self, name: str, email: str):
        self.id = 0
        self.name = name
        self.email = email
        self.active = True
        self.roles = []

    def activate(self) -> None:
        self.active = True

    def is_active(self) -> bool:
        return self.active


# Dataclass (Python 3.7+)
@dataclass
class Rectangle:
    width: int
    height: int
    color: str = "black"

    def get_area(self) -> int:
        return self.width * self.height


# NamedTuple
class Point2D(NamedTuple):
    x: int
    y: int
    label: Optional[str] = None


# TypedDict
class UserDict(TypedDict):
    id: int
    name: str
    email: str
    active: bool


# Enum
class UserRole(Enum):
    ADMIN = 0
    MODERATOR = 1
    USER = 2
    GUEST = 3


# Enum with methods
class Status(Enum):
    PENDING = "pending"
    ACTIVE = "active"
    COMPLETED = "completed"

    @classmethod
    def from_string(cls, value: str) -> 'Status':
        for status in cls:
            if status.value == value:
                return status
        return cls.PENDING

    def is_terminal(self) -> bool:
        return self == self.COMPLETED


# Abstract Base Class
from abc import ABC, abstractmethod

class Shape(ABC):
    color: str = "black"

    @abstractmethod
    def get_area(self) -> float:
        pass

    def set_color(self, color: str) -> None:
        self.color = color


class Circle(Shape):
    radius: float

    def __init__(self, radius: float):
        self.radius = radius

    def get_area(self) -> float:
        return 3.14159 * self.radius ** 2


class RectangleShape(Shape):
    width: float
    height: float

    def __init__(self, width: float, height: float):
        self.width = width
        self.height = height

    def get_area(self) -> float:
        return self.width * self.height


# Nested class
class Container:
    name: str
    items: List[int]

    class Iterator:
        index: int = 0
        _items: List[int]

        def __init__(self, items: List[int]):
            self._items = items
            self.index = 0

        def has_next(self) -> bool:
            return self.index < len(self._items)

        def next(self) -> int:
            if self.has_next():
                result = self._items[self.index]
                self.index += 1
                return result
            raise StopIteration

    def __init__(self, name: str):
        self.name = name
        self.items = []

    def add_item(self, item: int) -> None:
        self.items.append(item)

    def get_iterator(self) -> 'Container.Iterator':
        return Container.Iterator(self.items)


# Generic class
class Box:
    content: object = None
    empty: bool = True

    def set(self, item: object) -> None:
        self.content = item
        self.empty = False

    def get(self) -> object:
        return self.content


# Exception class
class CustomException(Exception):
    error_code: str

    def __init__(self, message: str, error_code: str):
        super().__init__(message)
        self.error_code = error_code


# Using attrs library
@attr.s(auto_attribs=True)
class Person:
    name: str
    age: int
    email: str = None

    def greet(self) -> str:
        return f"Hello, {self.name}!"


# SimpleNamespace
from types import SimpleNamespace

# This is runtime object, not defined in code
# config = SimpleNamespace(host="localhost", port=8080)


# Protocol (Python 3.8+)
from typing import Protocol

class Drawable(Protocol):
    def draw(self) -> None:
        ...

    def set_color(self, color: str) -> None:
        ...


# Final class (Python 3.8+)
from typing import Final

class Constants:
    PI: Final = 3.14159
    MAX_SIZE: Final = 1000


# Slotted class (memory optimization)
class SlottedClass:
    __slots__ = ('x', 'y', 'z')

    def __init__(self, x, y, z=0):
        self.x = x
        self.y = y
        self.z = z
