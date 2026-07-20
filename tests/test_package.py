from memento import __version__


def test_package_version() -> None:
    assert __version__ == "0.3.0rc6"
