from units import to_celsius


def test_freezing_point():
    assert to_celsius(32) == 0


def test_boiling_point():
    assert to_celsius(212) == 100
