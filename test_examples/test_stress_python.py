# STRESS TEST: Complex Python with dataclasses, enums, ABC, protocols, attrs
from __future__ import annotations
from dataclasses import dataclass, field, fields
from typing import (
    List, Dict, Optional, Union, Any, Callable,
    TypeVar, Generic, Protocol, TypedDict,
    NamedTuple, Final, Literal
)
from enum import Enum, Flag, IntEnum, auto
from abc import ABC, abstractmethod, ABCMeta
from functools import lru_cache
import attr
from collections import namedtuple

# === DATACLASSES ===
@dataclass
class SimpleDataclass:
    name: str
    value: int

@dataclass
class DataclassWithDefaults:
    required: str
    optional: int = 42
    with_factory: List[int] = field(default_factory=list)

@dataclass
class DataclassWithComplexTypes:
    items: List[str]
    mapping: Dict[str, int]
    optional: Optional[str] = None
    union_field: Union[int, str, float] = 0

@dataclass
class NestedDataclass:
    inner: SimpleDataclass
    list_of_inners: List[SimpleDataclass]
    mapping_of_inners: Dict[str, SimpleDataclass]

@dataclass(frozen=True)
class ImmutableDataclass:
    name: str
    value: int

@dataclass(slots=True)
class SlottedDataclass:
    name: str
    value: int

@dataclass
class DataclassWithValidators:
    name: str
    age: int
    
    def __post_init__(self):
        if self.age < 0:
            raise ValueError("Age cannot be negative")

@dataclass
class RecursiveDataclass:
    value: int
    children: List[RecursiveDataclass] = field(default_factory=list)

# === ENUMS ===
class Color(Enum):
    RED = 1
    GREEN = 2
    BLUE = 3

class Status(Flag):
    PENDING = 1
    ACTIVE = 2
    COMPLETED = 4
    FAILED = 8

class Priority(IntEnum):
    LOW = 1
    MEDIUM = 2
    HIGH = 3

class AutoEnum(Enum):
    VALUE_A = auto()
    VALUE_B = auto()
    VALUE_C = auto()

# === NAMED TUPLES ===
class Point(NamedTuple):
    x: float
    y: float

class Point3D(NamedTuple):
    x: float
    y: float
    z: float

class NamedRecord(NamedTuple):
    name: str
    values: List[int]
    metadata: Dict[str, Any]

# === TYPED DICT ===
class UserDict(TypedDict):
    name: str
    age: int
    email: Optional[str]

class ConfigDict(TypedDict, total=False):
    host: str
    port: int
    debug: bool

# === ABC CLASSES ===
class AbstractBase(ABC):
    @abstractmethod
    def abstract_method(self) -> None:
        pass
    
    def concrete_method(self) -> str:
        return "concrete"

class AbstractWithProperty(ABC):
    @property
    @abstractmethod
    def abstract_property(self) -> str:
        pass

class AbstractWithClassVar(ABC):
    class_var: str = "class variable"
    
    @abstractmethod
    def abstract_method(self) -> None:
        pass

# === PROTOCOLS ===
class Drawable(Protocol):
    def draw(self) -> None:
        ...
    
    def get_color(self) -> str:
        ...

class Comparable(Protocol[T]):
    def __lt__(self, other: T) -> bool:
        ...
    
    def __gt__(self, other: T) -> bool:
        ...

# === ATTRS CLASSES ===
@attr.s(auto_attribs=True)
class AttrsClass:
    name: str
    value: int = 42

@attr.s
class AttrsWithOptions:
    name: str = attr.ib()
    value: int = attr.ib(default=0)
    readonly: int = attr.ib(init=False, default=42)
    
    def method(self) -> str:
        return f"{self.name}: {self.value}"

@attr.s
class AttrsWithValidators:
    name: str = attr.ib(validator=attr.validators.instance_of(str))
    age: int = attr.ib(validator=attr.validators.in_range(0, 150))

@attr.s
class AttrsWithConverters:
    items: List[str] = attr.ib(converter=list)

@attr.s
class AttrsWithSlots:
    __slots__ = ('name', 'value')
    name: str
    value: int

# === NAMEDTUPLE FROM ATTRS ===
MyTuple = namedtuple('MyTuple', ['field1', 'field2', 'field3'])

# === GENERIC CLASSES ===
T = TypeVar('T')
U = TypeVar('U')

class GenericContainer(Generic[T]):
    def __init__(self, item: T):
        self.item = item
    
    def get(self) -> T:
        return self.item
    
    def set(self, item: T) -> None:
        self.item = item

class GenericTwoParams(Generic[T, U]):
    def __init__(self, first: T, second: U):
        self.first = first
        self.second = second
    
    def get_first(self) -> T:
        return self.first
    
    def get_second(self) -> U:
        return self.second

class GenericWithConstraint(Generic[T, U]):
    def __init__(self, items: List[T]):
        self.items = items

# === NESTED CLASSES ===
class OuterClass:
    class InnerClass:
        class DeepInnerClass:
            value: int = 42
    
    class StaticInner:
        name: str = "static"
        
        def method(self) -> str:
            return self.name

# === COMPLEX INHERITANCE ===
class BaseClass:
    base_field: str = "base"
    
    def base_method(self) -> str:
        return "base"

class MiddleClass(BaseClass):
    middle_field: str = "middle"
    
    def middle_method(self) -> str:
        return "middle"

class DerivedClass(MiddleClass):
    derived_field: str = "derived"
    
    def derived_method(self) -> str:
        return "derived"

class DiamondLeft(BaseClass):
    left_field: str = "left"

class DiamondRight(BaseClass):
    right_field: str = "right"

class DiamondDerived(DiamondLeft, DiamondRight):
    derived_field: str = "derived"

# === FINAL CLASSES ===
class FinalClass:
    """This class cannot be subclassed"""
    name: str = "final"
    
    def get_name(self) -> str:
        return self.name

# === PROTOCOLS WITH IMPLEMENTATIONS ===
class Point2D:
    def __init__(self, x: float, y: float):
        self.x = x
        self.y = y
    
    def draw(self) -> None:
        print(f"Drawing point at ({self.x}, {self.y})")
    
    def get_color(self) -> str:
        return "black"

class Point2DComparable(Point2D, Comparable[Point2D]):
    def __lt__(self, other: Point2D) -> bool:
        return (self.x ** 2 + self.y ** 2) < (other.x ** 2 + other.y ** 2)
    
    def __gt__(self, other: Point2D) -> bool:
        return (self.x ** 2 + self.y ** 2) > (other.x ** 2 + other.y ** 2)

# === TYPE ALIASES ===
IntList = List[int]
StringMap = Dict[str, str]
Callback = Callable[[int, str], bool]

# === COMPLEX TYPING PATTERNS ===
class TypeVarContainer(Generic[T]):
    def __init__(self):
        self._data: Dict[str, T] = {}
    
    def set(self, key: str, value: T) -> None:
        self._data[key] = value
    
    def get(self, key: str) -> Optional[T]:
        return self._data.get(key)
    
    def items(self) -> List[Tuple[str, T]]:
        return list(self._data.items())

# === LITERAL TYPES ===
class LiteralClass:
    mode: Literal["read", "write", "append"] = "read"
    
    def set_mode(self, new_mode: Literal["read", "write", "append"]) -> None:
        self.mode = new_mode

# === FINAL VARIABLES (TYPE ANNOTATION) ===
class Constants:
    MAX_SIZE: Final = 100
    DEFAULT_TIMEOUT: Final[int] = 30
    VALID_STATUSES: Final[List[str]] = ["active", "pending"]

# === RECURSIVE TYPE ===
class TreeNode:
    value: int
    left: Optional[TreeNode] = None
    right: Optional[TreeNode] = None
    
    def __init__(self, value: int, 
                 left: Optional[TreeNode] = None,
                 right: Optional[TreeNode] = None):
        self.value = value
        self.left = left
        self.right = right

# === PROTOCOL WITH SELF ===
class SelfProtocol(Protocol):
    def copy(self) -> SelfProtocol:
        ...

# === COMPLEX PROPERTY CLASS ===
class PropertyClass:
    def __init__(self):
        self._internal_value = 0
    
    @property
    def value(self) -> int:
        return self._internal_value
    
    @value.setter
    def value(self, new_value: int) -> None:
        if new_value < 0:
            raise ValueError("Value must be non-negative")
        self._internal_value = new_value
    
    @property
    def squared(self) -> int:
        return self._internal_value ** 2

# === CLASS WITH CLASSMETHODS ===
class ClassMethods:
    class_var: str = "class level"
    
    @classmethod
    def class_method(cls, value: str) -> str:
        return f"Class method: {value}"
    
    @staticmethod
    def static_method(value: int) -> int:
        return value * 2
    
    def instance_method(self) -> str:
        return f"Instance method: {self.class_var}"

# === OVERLOADED FUNCTION CLASS ===
class OverloadedFunctions:
    @overload
    def process(self, value: int) -> int:
        ...
    
    @overload
    def process(self, value: str) -> str:
        ...
    
    def process(self, value: Union[int, str]) -> Union[int, str]:
        if isinstance(value, int):
            return value * 2
        return value.upper()

# === CALLABLE CLASSES ===
class CallableClass:
    def __init__(self, multiplier: int):
        self.multiplier = multiplier
    
    def __call__(self, value: int) -> int:
        return value * self.multiplier

# === COMPLEX ABC ===
class Container(ABC, Generic[T]):
    @abstractmethod
    def add(self, item: T) -> None:
        pass
    
    @abstractmethod
    def remove(self, item: T) -> bool:
        pass
    
    @abstractmethod
    def contains(self, item: T) -> bool:
        pass
    
    @abstractmethod
    def size(self) -> int:
        pass

class ListContainer(Container[str]):
    def __init__(self):
        self._items: List[str] = []
    
    def add(self, item: str) -> None:
        self._items.append(item)
    
    def remove(self, item: str) -> bool:
        try:
            self._items.remove(item)
            return True
        except ValueError:
            return False
    
    def contains(self, item: str) -> bool:
        return item in self._items
    
    def size(self) -> int:
        return len(self._items)

# === MIXIN PATTERN ===
class JsonMixin:
    def to_json(self) -> str:
        import json
        return json.dumps(self.__dict__)
    
    @classmethod
    def from_json(cls, data: str):
        import json
        obj = cls()
        obj.__dict__.update(json.loads(data))
        return obj

class SerializableMixin:
    def serialize(self) -> bytes:
        import pickle
        return pickle.dumps(self)
    
    @classmethod
    def deserialize(cls, data: bytes):
        import pickle
        return pickle.loads(data)

class DataClass(JsonMixin, SerializableMixin):
    name: str = "default"
    value: int = 0

# === CHAINED PROPERTY ===
class ChainedProperty:
    def __init__(self):
        self._value = 0
    
    @property
    def value(self) -> int:
        return self._value
    
    @value.setter
    def value(self, new_value: int) -> None:
        self._value = new_value
    
    @property
    def doubled(self) -> int:
        return self._value * 2
    
    @property
    def tripled(self) -> int:
        return self._value * 3

# === DESCRIPTOR CLASSES ===
class ValidatedValue:
    def __init__(self, min_value: int, max_value: int):
        self.min_value = min_value
        self.max_value = max_value
    
    def __set_name__(self, owner: str, name: str):
        self.name = name
    
    def __get__(self, instance, owner):
        return instance.__dict__.get(self.name, 0)
    
    def __set__(self, instance, value):
        if not (self.min_value <= value <= self.max_value):
            raise ValueError(f"{self.name} must be between {self.min_value} and {self.max_value}")
        instance.__dict__[self.name] = value

class DescriptorClass:
    age: ValidatedValue = ValidatedValue(0, 150)
    score: ValidatedValue = ValidatedValue(0, 100)

# === COMPLEX GENERIC BOUNDS ===
T_co = TypeVar('T_co', covariant=True)
T_contra = TypeVar('T_contra', contravariant=True)

class CovariantWrapper(Generic[T_co]):
    def __init__(self, item: T_co):
        self.item = item
    
    def get(self) -> T_co:
        return self.item

class ContravariantConsumer(Generic[T_contra]):
    def __init__(self):
        self.items: List[T_contra] = []
    
    def consume(self, item: T_contra) -> None:
        self.items.append(item)
