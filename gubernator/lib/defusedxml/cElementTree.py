# defusedxml
#
# Copyright (c) 2013 by Christian Heimes <christian@python.org>
# Licensed to PSF under a Contributor Agreement.
# See http://www.python.org/psf/license for licensing details.
"""Defused xml.etree.cElementTree
"""
from __future__ import absolute_import

from xml.etree.cElementTree import TreeBuilder as _TreeBuilder
from xml.etree.cElementTree import parse as _parse
from xml.etree.cElementTree import tostring
# iterparse from ElementTree!
from xml.etree.ElementTree import iterparse as _iterparse

from .ElementTree import DefusedXMLParser, _IterParseIterator
from .common import _generate_etree_functions

__origin__ = "xml.etree.cElementTree"

XMLTreeBuilder = XMLParse = DefusedXMLParser

parse, iterparse, fromstring = _generate_etree_functions(DefusedXMLParser,
        _TreeBuilder, _IterParseIterator, _parse, _iterparse)
XML = fromstring
